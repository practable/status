package config

import (
	"sync"
	"time"

	jc "github.com/practable/jump/pkg/status"
	rc "github.com/practable/relay/pkg/status"
)

type Config struct {
	BasepathBook        string
	BasepathJump        string
	BasepathRelay       string
	EmailAuthType       string
	EmailFrom           string
	EmailHost           string
	EmailLink           string
	EmailPassword       string
	EmailPort           int
	EmailTo             []string
	EmailCc             []string
	EmailSubject        string
	HealthEvents        int
	HealthLastChecked   time.Duration
	HealthLastActive    time.Duration
	HealthStartup       time.Duration
	HealthLogEvery      time.Duration
	HostBook            string
	HostJump            string
	HostRelay           string
	Port                int
	QueryBookEvery      time.Duration
	ReconnectJumpEvery  time.Duration
	ReconnectRelayEvery time.Duration
	SecretBook          string
	SecretJump          string
	SecretRelay         string
	SendEmail           bool
	SchemeBook          string
	SchemeJump          string
	SchemeRelay         string
	TimeoutBook         time.Duration
}

// Status represents the overall status of the experiments
type Status struct {
	*sync.RWMutex
	Config Config
	// key is the topic_stub,m e.g. pend00 (and not the resource name r-pend00)
	Experiments map[string]Report
	LastCheck   map[string]map[string]bool // hold the last check of the streams
	Now         func() time.Time
}

type HealthyIssues struct {
	Healthy     bool
	JumpHealthy bool
	Issues      []string
}

type HealthEvent struct {
	Healthy  bool
	Issues   []string
	JumpOK   bool
	StreamOK map[string]bool
	When     time.Time
}

// Report represents the status
// when move to a signalling system
// Might want to do a daily check where all are woken up, to connect so we can report status?
type Report struct {
	Available           bool
	FirstChecked        time.Time //deprecate
	Healthy             bool
	HealthEvents        []HealthEvent // deprecate - will be on and off per booking in future??
	JumpOK              bool          //deprecate
	JumpHealthy         bool          // deprecate
	JumpReport          jc.Report     //deprecate
	LastCheckedJump     time.Time     // deprecate
	LastCheckedStreams  time.Time     // deprecate
	LastFoundInManifest time.Time     // deprecate
	ResourceName        string
	StreamOK            map[string]bool
	StreamReports       map[string]rc.Report //deprecate
	StreamRequired      map[string]bool      //deprecate
}

// New returns an Status with initialised maps
func New() *Status {
	return &Status{
		&sync.RWMutex{},
		Config{},
		make(map[string]Report),
		make(map[string]map[string]bool),
		func() time.Time { return time.Now() },
	}

}

// ClearExperiments removes all experiment reports
// this is intended for use only in testing, between test cases

func (s *Status) ClearExperiments() {
	s.Lock()
	defer s.Unlock()
	s.Experiments = make(map[string]Report)
}

func (s *Status) WithConfig(config Config) *Status {
	s.Lock()
	defer s.Unlock()
	s.Config = config
	return s
}

func (s *Status) SetNow(now func() time.Time) {
	s.Lock()
	defer s.Unlock()
	s.Now = now
}
