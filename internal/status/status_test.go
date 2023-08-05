package status

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	rt "github.com/go-openapi/runtime"
	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/mux"
	smtpmock "github.com/mocktools/go-smtp-mock/v2"
	"github.com/phayes/freeport"
	bc "github.com/practable/book/pkg/resource"
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
var resourcesYAML []byte
var resourcesJSON []byte
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

	resourcesYAML, err = ioutil.ReadFile("resources.yaml")
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
		panic(err)
	}

	resourcesJSON, err = yaml.YAMLToJSON(resourcesYAML)
	if err != nil {
		log.Printf("yaml.YAMLToJSON resources err   #%v ", err)
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

	/***************************************
	*  start mock book and load manifest
	**************************************/

	// Need a mock because
	// 0. cannot run book from pkg/book because the jwt middle layer for
	// status and book collide, and use the host value of the last started
	// service, in this case book rejects with "token aud does not match this host"
	// and instead wants the host that status has. This cannot really be solved
	// by tweaking the jwt library because we need both to use latest jwt/v4
	// 1. Starting book from the command line does not translate well to github actions
	// and would have different system time (time issue not a dealbreaker)

	// Auth? Skip for now, add if we encounter many PR that cause issues with auth
	// that we don't detect until integration testing.

	shutdownMockServers := make(chan struct{})
	idleConnsClosedBook := make(chan struct{})
	bmux := mux.NewRouter() //for path params //http.NewServeMux()
	bhost := ":" + strconv.Itoa(portBook)

	bam := make(map[string]bc.Status) //availability map for resources in the mock book

	bmux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	})
	bmux.HandleFunc("/api/v1/admin/resources", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, string(resourcesJSON))
	}).Methods("GET")

	bmux.HandleFunc("/api/v1/admin/resources/{name}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		name, ok := vars["name"]
		if !ok {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		status, ok := bam[name]
		if !ok {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		resp, err := json.Marshal(status)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, string(resp))
	}).Methods("GET")

	bmux.HandleFunc("/api/v1/admin/resources/{name}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		name, ok := vars["name"]
		if !ok {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		availableStr := r.URL.Query().Get("available")
		reason := r.URL.Query().Get("reason")
		available, err := strconv.ParseBool(availableStr)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		bam[name] = bc.Status{
			Available: available,
			Reason:    reason,
		}
		w.WriteHeader(http.StatusNoContent)
	}).Methods("PUT")

	bsrv := http.Server{
		Addr:    bhost,
		Handler: bmux,
	}

	go func() {
		if err := bsrv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("HTTP mock book server ListenAndServe: %v", err)
		}
	}()

	go func() {
		<-shutdownMockServers
		if err := bsrv.Shutdown(ctx); err != nil {
			// Error from closing listeners, or context timeout:
			log.Printf("HTTP mock book server Shutdown: %v", err)
		}
		close(idleConnsClosedBook)
	}()

	/***************************************************************************/
	// start dummy jump and relay in an elegant way
	// don't respond to access request until after tests finish,
	//  meanwhile we can mock responses using the channel in the stats clients
	/***************************************************************************/

	idleConnsClosedJump := make(chan struct{})
	idleConnsClosedRelay := make(chan struct{})

	jmux := http.NewServeMux()
	jmux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		<-shutdownMockServers
		fmt.Fprintf(w, "You can safely ignore this error")
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
		fmt.Fprintf(w, "You can safely ignore this error")
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
	*  define config for status server
	****************************/

	time.Sleep(100 * time.Millisecond) // let other services start

	s = config.New()
	s.Config = config.Config{
		BasepathBook:        "",
		BasepathJump:        "",
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
		HealthLastActive:    time.Duration(10 * time.Second),
		HealthLastChecked:   time.Minute,
		HealthLogEvery:      time.Hour,   //prevent interfering with test
		HealthStartup:       time.Minute, // won't cause actual delays
		HostBook:            "[::]:" + strconv.Itoa(portBook),
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

	/***************************
	*  Start status server
	****************************/

	// supply jump and relay clients so we can mock messages

	j = jc.New()
	r = rc.New()
	go Run(ctx, j, r, s) //doesn't include the API server

	time.Sleep(100 * time.Millisecond) //let status start

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

// TestStatus checks that reports are processed correctly
func TestStatus(t *testing.T) {

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
					   test00 data               -                  ok    no       no            U [missing required test00-st-data]
					   test01 data               data               no    no       yes         A   [missing jump]
					   test02 data               video              ok    no       no            U [missing required test02-st-data]
		                                                                                             stream ok but not the required on
					   test03 data               data video         ok    yes      yes               ok correct stream plus additional not in manifest that is ok
					   test04 data video         data               ok    no       no            U [missing required test04-st-video]
		                                                                                             missing required video stream, data stream healthy.
		                                                                                             Client connection to video stream must not cause healthy to be true
					   test05 data video         data video         ok    yes      yes         ok    all required streams present
					   test06 N/A                data video(Never)  ok    no       no             U  only one of the supplied streams is ok, the other never worked
					   test07 N/A                data video(3m)     ok    no       no             U  only one of the supplied streams is ok, the other has stopped
				       test08 N/A                none      client-only    n/a      n/a         --    should be ignored, and not appear in statistics on experiments

	*/

	loopTime := time.Date(2022, 11, 5, 0, 0, 0, 0, time.UTC)

	for i := 1; i < 10; i++ {

		setNow(t, loopTime)

		j.Status <- jr
		r.Status <- rr

		time.Sleep(time.Millisecond)

		loopTime = loopTime.Add(10 * time.Second)

	}

	if verbose {
		fmt.Printf("\n\nSET00\n%+v\n\n\n", s.Experiments)
	}

	if false { //used to generate contents for experiments.json, used by TestEmailBody
		s.Lock()
		sj, err := yaml.Marshal(s.Experiments)
		s.Unlock()
		assert.NoError(t, err)
		err = ioutil.WriteFile("experiments.yaml", sj, 0644)
		if err != nil {
			panic(err)
		}
	}

	// wait out the health startup of 1min, and then some, so we are starting next phase of test on an even 2min for ease of clock time editing

	// Check current status

	assert.Equal(t, false, s.Experiments["test00"].Available)
	assert.Equal(t, true, s.Experiments["test01"].Available)
	assert.Equal(t, false, s.Experiments["test02"].Available)
	assert.Equal(t, true, s.Experiments["test03"].Available)
	assert.Equal(t, false, s.Experiments["test04"].Available)
	assert.Equal(t, true, s.Experiments["test05"].Available)
	assert.Equal(t, false, s.Experiments["test06"].Available)
	assert.Equal(t, true, s.Experiments["test07"].Available)

	setNow(t, time.Date(2022, 11, 5, 0, 1, 55, 0, time.UTC)) //

	j.Status <- jr
	r.Status <- rr

	time.Sleep(10 * time.Millisecond)

	setNow(t, time.Date(2022, 11, 5, 0, 2, 0, 0, time.UTC)) // new time is 2min, longer than config.HealthStartup, and within config.HealthLast, so any healthy connections do not appear stale

	j.Status <- jr
	r.Status <- rr

	time.Sleep(10 * time.Millisecond)

	setNow(t, time.Date(2022, 11, 5, 0, 2, 5, 0, time.UTC)) //keep steps smaller than config.HealthLast else relay reports appear stale
	/*
		jr = reports.Jump["set01"].Reports
		rr = reports.Relay["set01"].Reports

		j.Status <- jr
		r.Status <- rr

		time.Sleep(10 * time.Millisecond)

		if verbose {
			fmt.Printf("\n\nSET01\n%+v\n\n\n", s.Experiments)
		}

		setNow(t, time.Date(2022, 11, 5, 0, 2, 10, 0, time.UTC))

		jr = reports.Jump["set02"].Reports
		rr = reports.Relay["set02"].Reports

		j.Status <- jr
		r.Status <- rr

		time.Sleep(100 * time.Millisecond)

		if verbose {
			fmt.Printf("\n\nSET02\n%+v\n\n\n", s.Experiments)
		}
	*/
	time.Sleep(100 * time.Millisecond)

	msg := SMTPServer.Messages()

	fmt.Printf("\n\nEmails\n%+v\n\n\n", msg)

	/*

		Subject: test 8 new health events

		System time: 2022-11-05 00:01:10 +0000 UTC
		There are 8 new health events (4 issues, 4 ok):
		test00 -- [missing jump, missing required test00-st-data]
		test01 -- [unhealthy test01-st-data]
		test02 -- [missing required test02-st-data]
		test04 -- [missing required test04-st-video]
		test03 ok
		test05 ok
		test06 ok
		test07 ok


		 All new and existing health issues:
		test00 -- [missing jump,missing required test00-st-data]
		test01 -- [unhealthy test01-st-data]
		test02 -- [missing required test02-st-data]
		test04 -- [missing required test04-st-video]



	*/

	//fmt.Printf("\n\nStatus\n%+v\n\n\n", s.Experiments)

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

func TestEmailBody(t *testing.T) {

	s = config.New()

	s.Config = config.Config{
		EmailCc:      []string{"cc@test.org"},
		EmailFrom:    "admin@test.org",
		EmailHost:    "localhost",
		EmailLink:    "https://app.test.org/tenant/status",
		EmailSubject: "test",
		EmailTo:      []string{"to@test.org"},
	}

	s.Now = func() time.Time { return time.Date(2022, 11, 5, 0, 2, 10, 0, time.UTC) }

	experimentsYAML, err := ioutil.ReadFile("experiments.yaml")
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
		panic(err)
	}

	var se map[string]config.Report

	err = yaml.Unmarshal(experimentsYAML, &se)

	assert.NoError(t, err)

	s.Experiments = se

	alerts := make(map[string]bool)
	alerts["test01"] = false

	systemAlerts := []string{}

	msg := EmailBody(s, alerts, systemAlerts)

	// the content of the health events is not quite right in these, but those are supplied as parameters, so this does not affect the validity of this test (TODO: update to latest format for cosmetic reasons)
	exp0 := "To: to@test.org\r\nCc: cc@test.org\r\nSubject: test 1 new health events \r\n\r\nSystem time: 2022-11-05 00:02:10 +0000 UTC\r\nThere are 1 new health events (1 issues, 0 ok): \r\ntest01 -- A   [unhealthy test01-st-data]\r\n\r\n\r\n All new and existing health issues:\r\ntest00 ><   U [missing jump, missing required test00-st-data]\r\ntest01 -- A   [unhealthy test01-st-data]\r\ntest02 ><   U [missing required test02-st-data]\r\ntest04 ><   U [missing required test04-st-video]\r\n\r\n\r\n For the latest complete status information, please go to https://app.test.org/tenant/status\r\n"

	assert.Equal(t, exp0, msg)

	systemAlerts = []string{"some system issue or other"}

	msg = EmailBody(s, alerts, systemAlerts)

	exp1 := "To: to@test.org\r\nCc: cc@test.org\r\nSubject: test 1 new health events \r\n\r\nSystem time: 2022-11-05 00:02:10 +0000 UTC\r\n\r\nSystem alerts:\r\nsome system issue or other\r\nThere are 1 new health events (1 issues, 0 ok): \r\ntest01 -- A   [unhealthy test01-st-data]\r\n\r\n\r\n All new and existing health issues:\r\ntest00 ><   U [missing jump, missing required test00-st-data]\r\ntest01 -- A   [unhealthy test01-st-data]\r\ntest02 ><   U [missing required test02-st-data]\r\ntest04 ><   U [missing required test04-st-video]\r\n\r\n\r\n For the latest complete status information, please go to https://app.test.org/tenant/status\r\n"
	assert.Equal(t, exp1, msg)

}
