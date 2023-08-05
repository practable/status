package status

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	rt "github.com/go-openapi/runtime"
	"github.com/golang-jwt/jwt"
	smtpmock "github.com/mocktools/go-smtp-mock/v2"
	"github.com/phayes/freeport"
	"github.com/practable/book/pkg/book"
	jc "github.com/practable/jump/pkg/status"
	rc "github.com/practable/relay/pkg/status"
	"github.com/practable/status/internal/config"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
)

var bookAdminAuth rt.ClientAuthInfoWriter
var ct time.Time
var ctp *time.Time
var debug bool
var j *jc.Status
var manifestYAML []byte
var manifestJSON []byte
var r *rc.Status
var s *config.Status
var SMTPServer *smtpmock.Server
var timeout time.Duration

type TestReports struct {
	Jump  map[string]JumpReports  `yaml:"jump" json:"jump"`
	Relay map[string]RelayReports `yaml:"relay" json:"relay"`
}

type JumpReports struct {
	Description string
	Reports     []jc.Report
}

type RelayReports struct {
	Description string
	Reports     []rc.Report
}

func setNow(t *testing.T, now time.Time) {
	t.Logf("Time now %s", now)
	ct = now //this updates the status server and jwt time functions via ctp
}

func init() {
	debug = true
	if debug {
		log.SetReportCaller(true)
		log.SetLevel(log.DebugLevel)
		log.SetFormatter(&log.TextFormatter{FullTimestamp: false, DisableColors: true})
		defer log.SetOutput(os.Stdout)

	} else {
		log.SetLevel(log.WarnLevel)
		var ignore bytes.Buffer
		logignore := bufio.NewWriter(&ignore)
		log.SetOutput(logignore)
	}

	// create JSON versions of the manifests
	var err error

	manifestYAML, err = ioutil.ReadFile("manifest.yaml")
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
		panic(err)
	}

	manifestJSON, err = yaml.YAMLToJSON(manifestYAML)

	if err != nil {
		panic(err)
	}

}
func TestMain(m *testing.M) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Configure services
	// we will run an instance of book, but mock jump and relay by inserting messages on the
	// status reports channel
	// we will run a mock smtp server to check email contents

	ports, err := freeport.GetFreePorts(5)
	if err != nil {
		panic(err)
	}

	portBook := ports[0]
	portJump := ports[1]
	portRelay := ports[2]
	portServe := ports[3] // for status
	portSMTP := ports[4]

	hostBook := "[::]:" + strconv.Itoa(portBook)
	hostJump := "[::]:" + strconv.Itoa(portJump)
	hostRelay := "[::]:" + strconv.Itoa(portRelay)
	hostSMTP := "[::]:" + strconv.Itoa(portSMTP)

	if debug {
		fmt.Printf("book: %s\n", hostBook)
		fmt.Printf("jump: %s\n", hostJump)
		fmt.Printf("relay: %s\n", hostRelay)
		fmt.Printf("SMTP: %s\n", hostSMTP)
	}

	schemeBook := "http"
	schemeJump := "http"
	schemeRelay := "http"

	secretBook := "bb" // suggest using a uuid in production
	secretJump := "jj"
	secretRelay := "rr"

	// Time
	ct = time.Date(2022, 11, 5, 0, 0, 0, 0, time.UTC) // change time later using SetNow()
	ctp = &ct
	// modify the time function used to verify the jwt token
	// so that we can set it from the current time
	jwt.TimeFunc = func() time.Time { return *ctp }

	/***************************
	*  configure & start book
	****************************/

	bookConfig := book.DefaultConfig()
	bookConfig.Port = portBook
	bookConfig.StoreSecret = secretBook
	bookConfig.RelaySecret = secretRelay
	bookConfig.Now = func() time.Time { return *ctp }

	go book.Run(ctx, bookConfig)

	bookAdminToken, err := book.AdminToken(bookConfig, 60, "someuser")
	if err != nil {
		panic(err)
	}

	time.Sleep(time.Second) //let book start!

	// upload manifest
	client := &http.Client{}
	bodyReader := bytes.NewReader(manifestJSON)

	req, err := http.NewRequest("PUT", schemeBook+"://"+hostBook+"/api/v1/admin/manifest", bodyReader)
	if err != nil {
		panic(err)
	}
	req.Header.Add("Authorization", bookAdminToken)
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode != 200 {
		log.Fatalf("Book manifest did not load %v", resp)
		fmt.Printf("\n\n%v\n\n", string(manifestJSON))

	}

	/***************************************************************************/
	// start dummy jump and relay in an elegant way
	// don't respond to access request until after tests finish,
	//  meanwhile we can mock responses using the channel in the stats clients
	/***************************************************************************/

	idleConnsClosedJump := make(chan struct{})
	idleConnsClosedRelay := make(chan struct{})
	shutdownMockServers := make(chan struct{})

	jmux := http.NewServeMux()
	jmux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		<-shutdownMockServers
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	})
	jhost := ":" + strconv.Itoa(portJump)

	jsrv := http.Server{
		Addr:    jhost,
		Handler: jmux,
	}

	go func() {
		if err := jsrv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("HTTP jumpserver ListenAndServe: %v", err)
		}
	}()

	go func() {
		<-shutdownMockServers
		if err := jsrv.Shutdown(ctx); err != nil {
			// Error from closing listeners, or context timeout:
			log.Printf("HTTP mock jump server Shutdown: %v", err)
		}
		close(idleConnsClosedJump)
	}()

	rmux := http.NewServeMux()
	rmux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		<-shutdownMockServers
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	})
	rhost := ":" + strconv.Itoa(portRelay)

	rsrv := http.Server{
		Addr:    rhost,
		Handler: rmux,
	}

	go func() {
		if err := rsrv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("HTTP relay server ListenAndServe: %v", err)
		}
	}()

	go func() {
		<-shutdownMockServers
		if err := rsrv.Shutdown(ctx); err != nil {
			// Error from closing listeners, or context timeout:
			log.Printf("HTTP mock relay server Shutdown: %v", err)
		}
		close(idleConnsClosedRelay)
	}()

	/***************************
	*  start mock smtp
	****************************/

	SMTPServer = smtpmock.New(smtpmock.ConfigurationAttr{
		PortNumber: portSMTP,
	})

	if err := SMTPServer.Start(); err != nil {
		panic(err)
	}

	/***************************
	*  start status server
	****************************/

	time.Sleep(time.Second) // let other services start

	s = config.New()
	s.Config = config.Config{
		BasepathBook:        "/api/v1",
		BasepathJump:        "/api/v1",
		BasepathRelay:       "",
		EmailAuthType:       "none",
		EmailCc:             []string{"cc@test.org"},
		EmailFrom:           "admin@test.org",
		EmailHost:           "localhost",
		EmailLink:           "http://[::]:" + strconv.Itoa(portServe),
		EmailPassword:       "",
		EmailPort:           portSMTP,
		EmailSubject:        "test",
		EmailTo:             []string{"to@test.org"},
		HealthEvents:        10,
		HealthLast:          time.Duration(10 * time.Second),
		HealthLogEvery:      time.Hour,   //prevent interfering with test
		HealthStartup:       time.Minute, // won't cause actual delays
		HostBook:            hostBook,
		HostJump:            hostJump,
		HostRelay:           hostRelay,
		Port:                portServe,
		QueryBookEvery:      time.Hour,
		ReconnectJumpEvery:  time.Hour,
		ReconnectRelayEvery: time.Hour,
		SchemeBook:          schemeBook,
		SchemeJump:          schemeJump,
		SchemeRelay:         schemeRelay,
		SecretBook:          secretBook,
		SecretJump:          secretJump,
		SecretRelay:         secretRelay,
		SendEmail:           true,
		TimeoutBook:         time.Minute,
	}
	s.Now = func() time.Time { return *ctp }

	// supply jump and relay clients so we can mock messages

	j = jc.New()
	r = rc.New()
	go Run(ctx, j, r, s) //doesn't include the API server

	time.Sleep(time.Second) //let status start

	exitVal := m.Run()

	close(shutdownMockServers)

	if err := SMTPServer.Stop(); err != nil {
		fmt.Println(err) // print the error, but don't fail the test
	}

	<-idleConnsClosedJump
	<-idleConnsClosedRelay

	os.Exit(exitVal)
}

func TestGetID(t *testing.T) {

	id, err := getID("pend00-st-data")
	assert.NoError(t, err)
	assert.Equal(t, "pend00", id)

	id, err = getID("")

	assert.Error(t, err)
	assert.Equal(t, "", id)
}

func TestGetStream(t *testing.T) {

	stream, err := getStream("pend00-st-data")
	assert.NoError(t, err)
	assert.Equal(t, "data", stream)

	stream, err = getStream("")

	assert.Error(t, err)
	assert.Equal(t, "", stream)

}

// TestAllOK checks that with good
func TestAllOK(t *testing.T) {

	setNow(t, time.Date(2022, 11, 5, 0, 0, 0, 0, time.UTC))

	assert.Equal(t, time.Date(2022, 11, 5, 0, 0, 0, 0, time.UTC), s.Now())

	reportsYAML, err := ioutil.ReadFile("reports.yaml")
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
		panic(err)
	}

	var reports TestReports

	err = yaml.Unmarshal(reportsYAML, &reports)

	assert.NoError(t, err)

	var jr []jc.Report
	var rr []rc.Report

	jr = reports.Jump["set00"].Reports
	rr = reports.Relay["set00"].Reports

	verbose := false

	if verbose {
		fmt.Printf("\n\n%v\n\n", jr)
		fmt.Printf("\n\n%v\n\n", rr)
	}

	/*

			   Initial Status

			   expt   required streams   supplied streams   jump  healthy  available   comment
			   test00 data               -                  ok    no       no          no streams
			   test01 data               data               no    no       yes         required stream ok
			   test02 data               video              ok    no       no          stream ok but not the required on
			   test03 data               data video         ok    yes      yes         correct stream plus additional not in manifest that is ok
			   test04 data video         data               ok    no       no          missing required video stream, data stream healthy. Client connection to video stream must not cause healthy to be true
			   test05 data video         data video         ok    yes      yes         all required streams present
			   test06 N/A                data video(Never)  ok    no       no          only one of the supplied streams is ok, the other never worked
			   test07 N/A                data video(3m)     ok    no       no          only one of the supplied streams is ok, the other has stopped
		       test08 N/A                none      client-only    n/a      n/a        should be ignored, and not appear in statistics on experiments

	*/

	j.Status <- jr
	r.Status <- rr

	time.Sleep(100 * time.Millisecond)

	if verbose {

		fmt.Printf("\n\nSET00\n%+v\n\n\n", s.Experiments)
	}

	setNow(t, time.Date(2022, 11, 5, 0, 0, 5, 0, time.UTC)) //keep steps smaller than config.HealthLast else relay reports appear stale

	jr = reports.Jump["set01"].Reports
	rr = reports.Relay["set01"].Reports

	j.Status <- jr
	r.Status <- rr

	time.Sleep(100 * time.Millisecond)

	if verbose {

		fmt.Printf("\n\nSET01\n%+v\n\n\n", s.Experiments)
	}

	setNow(t, time.Date(2022, 11, 5, 0, 0, 10, 0, time.UTC))

	jr = reports.Jump["set02"].Reports
	rr = reports.Relay["set02"].Reports

	j.Status <- jr
	r.Status <- rr

	time.Sleep(100 * time.Millisecond)

	if verbose {

		fmt.Printf("\n\nSET02\n%+v\n\n\n", s.Experiments)
	}

	setNow(t, time.Date(2022, 11, 5, 0, 0, 15, 0, time.UTC))

	jr = reports.Jump["set03"].Reports
	rr = reports.Relay["set03"].Reports

	j.Status <- jr
	r.Status <- rr

	time.Sleep(time.Second)

	if verbose {

		fmt.Printf("\n\nSET03\n%+v\n\n\n", s.Experiments)
	}

	setNow(t, time.Date(2022, 11, 5, 0, 0, 20, 0, time.UTC))

	time.Sleep(100 * time.Millisecond)

	msg := SMTPServer.Messages()

	fmt.Printf("\n\nEmails\n%+v\n\n\n", msg)

	// prepare reports
	// send reports
	// check no health events yet
	// wait for just longer than health_startup
	// resend reports
	// check initial health events are accurate
	// check errors are correct
	// check email is correctly sent

	/* Second status

	    advance time by an hour

		test04 gains video stream, becomes ok
		test05 loses data stream, becomes unhealthy

		email should
		report test04 ok, test05 issues

		include test00, test01, test05, test06, test07 in list of all issues

	*/
}
