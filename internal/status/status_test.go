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

var debug bool
var ct time.Time
var ctp *time.Time
var timeout time.Duration
var s *config.Status
var bookAdminAuth rt.ClientAuthInfoWriter

var manifestYAML []byte
var manifestJSON []byte

func setNow(s *config.Status, now time.Time) {
	ct = now //this updates the jwt time function via ctp
	s.SetNow(func() time.Time { return now })
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

	SMTPServer := smtpmock.New(smtpmock.ConfigurationAttr{
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
		EmailCc:             []string{"cc@test.org"},
		EmailFrom:           "admin@test.org",
		EmailHost:           hostSMTP,
		EmailLink:           "http://[::]:" + strconv.Itoa(portServe),
		EmailPassword:       "",
		EmailPort:           portSMTP,
		EmailSubject:        "test",
		EmailTo:             []string{"to@test.org"},
		HealthEvents:        10,
		HealthLast:          time.Duration(1 * time.Second),
		HealthLogEvery:      time.Duration(1 * time.Second),
		HealthStartup:       time.Duration(1 * time.Second),
		HostBook:            hostBook,
		HostJump:            hostJump,
		HostRelay:           hostRelay,
		Port:                portServe,
		QueryBookEvery:      time.Duration(1 * time.Second),
		ReconnectJumpEvery:  time.Duration(1 * time.Hour),
		ReconnectRelayEvery: time.Duration(1 * time.Hour),
		SchemeBook:          schemeBook,
		SchemeJump:          schemeJump,
		SchemeRelay:         schemeRelay,
		SecretBook:          secretBook,
		SecretJump:          secretJump,
		SecretRelay:         secretRelay,
		SendEmail:           true,
		TimeoutBook:         time.Minute,
	}

	// supply jump and relay clients so we can mock messages

	j := jc.New()
	r := rc.New()
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
