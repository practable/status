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

// lock s before using this function to avoid race, because this var is
// shared with the status object (and unlock after)
func setNow(t *testing.T, now time.Time) {
	if debug {
		t.Logf("Time now %s", now)
	}
	ct = now //this updates the status server and jwt time functions via ctp
}

func init() {
	debug = false
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
		EmailLink:           "https://app.test.org/tenant/status", //don't include port because it breaks email tests by changing every run
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

	s.Lock()
	setNow(t, time.Date(2022, 11, 5, 0, 0, 0, 0, time.UTC))
	s.Unlock()

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

		s.Lock()
		setNow(t, loopTime)
		s.Unlock()

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
	s.Lock()
	assert.Equal(t, true, s.Experiments["test00"].JumpOK)
	assert.Equal(t, false, s.Experiments["test01"].JumpOK)
	assert.Equal(t, true, s.Experiments["test02"].JumpOK)
	assert.Equal(t, true, s.Experiments["test03"].JumpOK)
	assert.Equal(t, true, s.Experiments["test04"].JumpOK)
	assert.Equal(t, true, s.Experiments["test05"].JumpOK)
	assert.Equal(t, true, s.Experiments["test06"].JumpOK)
	assert.Equal(t, true, s.Experiments["test07"].JumpOK)

	assert.Equal(t, false, s.Experiments["test00"].Available)
	assert.Equal(t, true, s.Experiments["test01"].Available)
	assert.Equal(t, false, s.Experiments["test02"].Available)
	assert.Equal(t, true, s.Experiments["test03"].Available)
	assert.Equal(t, false, s.Experiments["test04"].Available)
	assert.Equal(t, true, s.Experiments["test05"].Available)
	assert.Equal(t, false, s.Experiments["test06"].Available)
	assert.Equal(t, false, s.Experiments["test07"].Available)

	assert.Equal(t, false, s.Experiments["test00"].StreamOK["test00-st-data"])
	assert.Equal(t, true, s.Experiments["test01"].StreamOK["test01-st-data"])
	assert.Equal(t, false, s.Experiments["test02"].StreamOK["test02-st-data"])
	assert.Equal(t, true, s.Experiments["test03"].StreamOK["test03-st-data"])
	assert.Equal(t, true, s.Experiments["test03"].StreamOK["test03-st-video"])
	assert.Equal(t, true, s.Experiments["test04"].StreamOK["test04-st-data"])
	assert.Equal(t, true, s.Experiments["test05"].StreamOK["test05-st-data"])
	assert.Equal(t, true, s.Experiments["test05"].StreamOK["test05-st-video"])
	assert.Equal(t, true, s.Experiments["test06"].StreamOK["test06-st-data"])
	assert.Equal(t, false, s.Experiments["test06"].StreamOK["test06-st-video"])
	assert.Equal(t, true, s.Experiments["test07"].StreamOK["test07-st-data"])
	assert.Equal(t, false, s.Experiments["test07"].StreamOK["test07-st-video"])

	// these checks were helping with debugging the reports.yaml file
	// you can't set Never in the reports.yaml file - instead set Last to the string "Never"
	assert.Equal(t, true, s.Experiments["test06"].StreamReports["test06-st-video"].Stats.Tx.Never)
	assert.Equal(t, false, s.Experiments["test07"].StreamReports["test07-st-video"].Stats.Tx.Never)

	if debug {
		t.Logf("test06-st-video.Tx: %+v", s.Experiments["test06"].StreamReports["test06-st-video"].Stats.Tx)
		t.Logf("test07-st-video.Tx: %+v", s.Experiments["test07"].StreamReports["test07-st-video"].Stats.Tx)
	}
	s.Unlock()

	/***********

	 SET01

	***********/

	jr = reports.Jump["set01"].Reports

	rr = reports.Relay["set01"].Reports

	for i := 1; i < 10; i++ { // must be longer than config.HealthLastChecked (1m) for test02's jump entry to expire
		s.Lock()
		setNow(t, loopTime)
		s.Unlock()

		j.Status <- jr
		r.Status <- rr

		time.Sleep(time.Millisecond)

		loopTime = loopTime.Add(10 * time.Second)

	}

	if verbose {
		fmt.Printf("\n\nSET01\n%+v\n\n\n", s.Experiments)
	}

	s.Lock()
	assert.Equal(t, true, s.Experiments["test00"].JumpOK)
	assert.Equal(t, false, s.Experiments["test01"].JumpOK)
	assert.Equal(t, false, s.Experiments["test02"].JumpOK)
	assert.Equal(t, true, s.Experiments["test03"].JumpOK)
	assert.Equal(t, true, s.Experiments["test04"].JumpOK)
	assert.Equal(t, true, s.Experiments["test05"].JumpOK)
	assert.Equal(t, true, s.Experiments["test06"].JumpOK)
	assert.Equal(t, true, s.Experiments["test07"].JumpOK)

	assert.Equal(t, false, s.Experiments["test00"].Available)
	assert.Equal(t, true, s.Experiments["test01"].Available)
	assert.Equal(t, false, s.Experiments["test02"].Available)
	assert.Equal(t, true, s.Experiments["test03"].Available)
	assert.Equal(t, false, s.Experiments["test04"].Available)
	assert.Equal(t, true, s.Experiments["test05"].Available)
	assert.Equal(t, false, s.Experiments["test06"].Available)
	assert.Equal(t, false, s.Experiments["test07"].Available)

	assert.Equal(t, false, s.Experiments["test00"].StreamOK["test00-st-data"])
	assert.Equal(t, true, s.Experiments["test01"].StreamOK["test01-st-data"])
	assert.Equal(t, false, s.Experiments["test02"].StreamOK["test02-st-data"])
	assert.Equal(t, true, s.Experiments["test03"].StreamOK["test03-st-data"])
	assert.Equal(t, true, s.Experiments["test03"].StreamOK["test03-st-video"])
	assert.Equal(t, true, s.Experiments["test04"].StreamOK["test04-st-data"])
	assert.Equal(t, false, s.Experiments["test05"].StreamOK["test05-st-data"])
	assert.Equal(t, true, s.Experiments["test05"].StreamOK["test05-st-video"])
	assert.Equal(t, true, s.Experiments["test06"].StreamOK["test06-st-data"])
	assert.Equal(t, false, s.Experiments["test06"].StreamOK["test06-st-video"])
	assert.Equal(t, true, s.Experiments["test07"].StreamOK["test07-st-data"])
	assert.Equal(t, false, s.Experiments["test07"].StreamOK["test07-st-video"])
	s.Unlock()

	jr = reports.Jump["set02"].Reports
	rr = reports.Relay["set02"].Reports

	for i := 1; i < 10; i++ {
		s.Lock()
		setNow(t, loopTime)
		s.Unlock()

		j.Status <- jr
		r.Status <- rr

		time.Sleep(time.Millisecond)

		loopTime = loopTime.Add(10 * time.Second)

	}

	time.Sleep(10 * time.Millisecond)

	s.Lock()
	assert.Equal(t, true, s.Experiments["test00"].JumpOK)
	assert.Equal(t, true, s.Experiments["test01"].JumpOK)
	assert.Equal(t, true, s.Experiments["test02"].JumpOK)
	assert.Equal(t, true, s.Experiments["test03"].JumpOK)
	assert.Equal(t, true, s.Experiments["test04"].JumpOK)
	assert.Equal(t, true, s.Experiments["test05"].JumpOK)
	assert.Equal(t, true, s.Experiments["test06"].JumpOK)
	assert.Equal(t, true, s.Experiments["test07"].JumpOK)

	assert.Equal(t, true, s.Experiments["test00"].Available)
	assert.Equal(t, true, s.Experiments["test01"].Available)
	assert.Equal(t, true, s.Experiments["test02"].Available)
	assert.Equal(t, true, s.Experiments["test03"].Available)
	assert.Equal(t, true, s.Experiments["test04"].Available)
	assert.Equal(t, true, s.Experiments["test05"].Available)
	assert.Equal(t, false, s.Experiments["test06"].Available) //not in manifest, can't be available
	assert.Equal(t, false, s.Experiments["test07"].Available) //not in manifest, can't be available

	assert.Equal(t, true, s.Experiments["test00"].StreamOK["test00-st-data"])
	assert.Equal(t, true, s.Experiments["test01"].StreamOK["test01-st-data"])
	assert.Equal(t, true, s.Experiments["test02"].StreamOK["test02-st-data"])
	assert.Equal(t, true, s.Experiments["test03"].StreamOK["test03-st-data"])
	assert.Equal(t, true, s.Experiments["test03"].StreamOK["test03-st-video"])
	assert.Equal(t, true, s.Experiments["test04"].StreamOK["test04-st-data"])
	assert.Equal(t, true, s.Experiments["test05"].StreamOK["test05-st-data"])
	assert.Equal(t, true, s.Experiments["test05"].StreamOK["test05-st-video"])
	assert.Equal(t, true, s.Experiments["test06"].StreamOK["test06-st-data"])
	assert.Equal(t, true, s.Experiments["test06"].StreamOK["test06-st-video"])
	assert.Equal(t, true, s.Experiments["test07"].StreamOK["test07-st-data"])
	assert.Equal(t, true, s.Experiments["test07"].StreamOK["test07-st-video"])
	s.Unlock()

	time.Sleep(100 * time.Millisecond)

	msg := SMTPServer.Messages()

	msgs := []string{}

	for _, v := range msg {
		msgs = append(msgs, v.MsgRequest())
	}

	msg0 := "To: to@test.org\r\nCc: cc@test.org\r\nSubject: test 8 new health events \r\n\r\nSystem time: 2022-11-05 00:01:00 +0000 UTC\r\nThere are 8 new health events (6 issues, 2 ok): \r\ntest00 ><   U [missing required test00-st-data]\r\ntest01 -- A   [missing jump]\r\ntest02 ><   U [missing required test02-st-data]\r\ntest04 ><   U [missing required test04-st-video]\r\ntest06 ><   U [unhealthy test06-st-video]\r\ntest07 ><   U [unhealthy test07-st-video]\r\ntest03 ok A   \r\ntest05 ok A   \r\n\r\n\r\n All new and existing health issues:\r\ntest00 ><   U [missing required test00-st-data]\r\ntest01 -- A   [missing jump]\r\ntest02 ><   U [missing required test02-st-data]\r\ntest04 ><   U [missing required test04-st-video]\r\ntest06 ><   U [unhealthy test06-st-video]\r\ntest07 ><   U [unhealthy test07-st-video]\r\n\r\n\r\n For the latest complete status information, please go to https://app.test.org/tenant/status\r\n"
	msg1 := "To: to@test.org\r\nCc: cc@test.org\r\nSubject: test 1 new health events \r\n\r\nSystem time: 2022-11-05 00:01:30 +0000 UTC\r\nThere are 1 new health events (1 issues, 0 ok): \r\ntest05 -- A   [unhealthy test05-st-data]\r\n\r\n\r\n All new and existing health issues:\r\ntest00 ><   U [missing required test00-st-data]\r\ntest01 -- A   [missing jump]\r\ntest02 ><   U [missing required test02-st-data]\r\ntest04 ><   U [missing required test04-st-video]\r\ntest05 -- A   [unhealthy test05-st-data]\r\ntest06 ><   U [unhealthy test06-st-video]\r\ntest07 ><   U [unhealthy test07-st-video]\r\n\r\n\r\n For the latest complete status information, please go to https://app.test.org/tenant/status\r\n"
	msg2 := "To: to@test.org\r\nCc: cc@test.org\r\nSubject: test 1 new health events \r\n\r\nSystem time: 2022-11-05 00:02:30 +0000 UTC\r\nThere are 1 new health events (1 issues, 0 ok): \r\ntest02 ><   U [missing jump, missing required test02-st-data]\r\n\r\n\r\n All new and existing health issues:\r\ntest00 ><   U [missing required test00-st-data]\r\ntest01 -- A   [missing jump]\r\ntest02 ><   U [missing jump, missing required test02-st-data]\r\ntest04 ><   U [missing required test04-st-video]\r\ntest05 -- A   [unhealthy test05-st-data]\r\ntest06 ><   U [unhealthy test06-st-video]\r\ntest07 ><   U [unhealthy test07-st-video]\r\n\r\n\r\n For the latest complete status information, please go to https://app.test.org/tenant/status\r\n"
	msg3 := "To: to@test.org\r\nCc: cc@test.org\r\nSubject: test 5 new health events \r\n\r\nSystem time: 2022-11-05 00:03:00 +0000 UTC\r\nThere are 5 new health events (0 issues, 5 ok): \r\ntest00 ok A   \r\ntest04 ok A   \r\ntest05 ok A   \r\ntest06 ok   U  \r\ntest07 ok   U  \r\n\r\n\r\n All new and existing health issues:\r\ntest01 -- A   [missing jump]\r\ntest02 -- A   [missing jump, missing required test02-st-data]\r\n\r\n\r\n For the latest complete status information, please go to https://app.test.org/tenant/status\r\n"
	msg4 := "To: to@test.org\r\nCc: cc@test.org\r\nSubject: test 2 new health events \r\n\r\nSystem time: 2022-11-05 00:03:00 +0000 UTC\r\nThere are 2 new health events (0 issues, 2 ok): \r\ntest01 ok A   \r\ntest02 ok A   \r\n\r\n\r\n All new and existing health issues:\r\n\r\n\r\n For the latest complete status information, please go to https://app.test.org/tenant/status\r\n"

	assert.Equal(t, msg0, msgs[0])
	assert.Equal(t, msg1, msgs[1])
	assert.Equal(t, msg2, msgs[2])
	assert.Equal(t, msg3, msgs[3])
	assert.Equal(t, msg4, msgs[4])
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
	exp0 := "To: to@test.org\r\nCc: cc@test.org\r\nSubject: test 1 new health events \r\n\r\nSystem time: 2022-11-05 00:02:10 +0000 UTC\r\nThere are 1 new health events (1 issues, 0 ok): \r\ntest01 -- A   [missing jump]\r\n\r\n\r\n All new and existing health issues:\r\ntest00 ><   U [missing required test00-st-data]\r\ntest01 -- A   [missing jump]\r\ntest02 ><   U [missing required test02-st-data]\r\ntest04 ><   U [missing required test04-st-video]\r\ntest06 ><   U [unhealthy test06-st-video]\r\ntest07 ><   U [unhealthy test07-st-video]\r\n\r\n\r\n For the latest complete status information, please go to https://app.test.org/tenant/status\r\n"

	assert.Equal(t, exp0, msg)

	systemAlerts = []string{"some system issue or other"}

	msg = EmailBody(s, alerts, systemAlerts)

	exp1 := "To: to@test.org\r\nCc: cc@test.org\r\nSubject: test 1 new health events \r\n\r\nSystem time: 2022-11-05 00:02:10 +0000 UTC\r\n\r\nSystem alerts:\r\nsome system issue or other\r\nThere are 1 new health events (1 issues, 0 ok): \r\ntest01 -- A   [missing jump]\r\n\r\n\r\n All new and existing health issues:\r\ntest00 ><   U [missing required test00-st-data]\r\ntest01 -- A   [missing jump]\r\ntest02 ><   U [missing required test02-st-data]\r\ntest04 ><   U [missing required test04-st-video]\r\ntest06 ><   U [unhealthy test06-st-video]\r\ntest07 ><   U [unhealthy test07-st-video]\r\n\r\n\r\n For the latest complete status information, please go to https://app.test.org/tenant/status\r\n"
	assert.Equal(t, exp1, msg)

}
