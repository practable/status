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
	EmailFrom           string
	EmailHost           string
	EmailLink           string
	EmailPassword       string
	EmailPort           int
	EmailTo             []string
	EmailCc             []string
	EmailSubject        string
	HealthEvents        int
	HealthLast          time.Duration
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
}

type HealthyIssues struct {
	Healthy bool
	Issues  []string
}

type HealthEvent struct {
	Healthy  bool
	Issues   []string
	JumpOK   bool
	StreamOK map[string]bool
	When     time.Time
}

// Report represents the status
type Report struct {
	Available           bool
	FirstChecked        time.Time
	Healthy             bool
	HealthEvents        []HealthEvent
	JumpOK              bool
	JumpReport          jc.Report
	LastCheckedJump     time.Time
	LastCheckedStreams  time.Time
	LastFoundInManifest time.Time
	ResourceName        string
	StreamOK            map[string]bool
	StreamReports       map[string]rc.Report
	StreamRequired      map[string]bool
}

// New returns an Status with initialised maps
func New() *Status {
	return &Status{
		&sync.RWMutex{},
		Config{},
		make(map[string]Report),
	}

}

func (s *Status) WithConfig(config Config) *Status {
	s.Config = config
	return s
}
