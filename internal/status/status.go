package status

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"net/smtp"

	jc "github.com/practable/jump/pkg/status"
	rc "github.com/practable/relay/pkg/status"
	"github.com/practable/relay/pkg/token"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

//ro "github.com/practable/status/internal/relay/operations"
//rm "github.com/practable/status/internal/relay/models"

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
	HostBook            string
	HostJump            string
	HostRelay           string
	Port                int
	ReconnectJumpEvery  int
	ReconnectRelayEvery int
	SecretBook          string
	SecretJump          string
	SecretRelay         string
	SendEmail           bool
	SchemeBook          string
	SchemeJump          string
	SchemeRelay         string
}

// Status represents the overall status of the experiments
type Status struct {
	*sync.RWMutex
	Config      Config
	Experiments map[string]Report
}

type HealthyIssues struct {
	Healthy bool
	Issues  []string
}

type HealthEvent struct {
	Healthy  bool
	When     time.Time
	JumpOK   bool
	StreamOK map[string]bool
	Issues   []string
}

// Report represents the status
type Report struct {
	FirstChecked       time.Time
	LastCheckedJump    time.Time
	LastCheckedStreams time.Time
	Healthy            bool
	HealthEvents       []HealthEvent
	JumpOK             bool
	JumpReport         jc.Report
	StreamOK           map[string]bool
	StreamReports      map[string]rc.Report
	StreamRequired     map[string]bool
}

func NewReport() Report {

	return Report{
		FirstChecked:   time.Now(),
		Healthy:        false,
		HealthEvents:   []HealthEvent{},
		JumpOK:         false,
		JumpReport:     jc.Report{},
		StreamOK:       make(map[string]bool),
		StreamReports:  make(map[string]rc.Report),
		StreamRequired: make(map[string]bool),
	}

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

// just pass in a context?
func Serve(ctx context.Context, wg *sync.WaitGroup, config Config) {

	defer wg.Done()

	// connect to stats channel
	// update stats data structure
	// start API server
	// report stats when GET received
	// extend to include jump
	// figure out IP to make sure clients don't fool us into thinking host is present
	// extend to check book for current availability status
	s := New().WithConfig(config)
	r := rc.New()
	j := jc.New()

	go connectRelay(ctx, config, r)
	go connectJump(ctx, config, j)

	go s.processRelay(ctx, r)
	go s.processJump(ctx, j)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				fmt.Printf("\nSTATUS\n*******\n")
				for k, v := range s.Experiments {
					fmt.Printf("%s:%+v\n", k, v)
				}
			}
		}

	}()

	<-ctx.Done()
	log.Trace("Status done")

}

func connectRelay(ctx context.Context, config Config, r *rc.Status) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// make token for relay stats connection for duration config.ReconnectRelayEvery
			iat := time.Now()
			nbf := time.Now()
			exp := time.Now().Add(time.Second * time.Duration(config.ReconnectRelayEvery))
			log.WithFields(log.Fields{"iat": iat, "nbf": nbf, "exp": exp}).Trace("Token times")
			aud := config.SchemeRelay + "://" + path.Join(config.HostRelay, config.BasepathRelay)
			bid := "status-server"
			connectionType := "session"
			scopes := []string{"read"}
			topic := "stats"
			token, err := token.New(iat, nbf, exp, scopes, aud, bid, connectionType, config.SecretRelay, topic)
			log.Tracef("token: [%s]", token)
			if err != nil {
				log.WithField("error", err.Error()).Error("Relay stats token generation failed")
				time.Sleep(5 * time.Second) //rate-limit retries if there is a long standing issue
				break
			}
			ctxStats, cancel := context.WithTimeout(ctx, time.Second*time.Duration(config.ReconnectRelayEvery))
			to := aud + "/" + connectionType + "/" + topic
			log.Tracef("to: [%s]", to)
			r.Connect(ctxStats, to, token)
			cancel() // call to save leaking, even though cancelled before getting to here
		}
	}
}

func connectJump(ctx context.Context, config Config, j *jc.Status) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// make token for relay stats connection for duration config.ReconnectRelayEvery
			iat := time.Now()
			nbf := time.Now()
			exp := time.Now().Add(time.Second * time.Duration(config.ReconnectJumpEvery))
			log.WithFields(log.Fields{"iat": iat, "nbf": nbf, "exp": exp}).Trace("Token times")
			aud := config.SchemeJump + "://" + path.Join(config.HostJump, config.BasepathJump)
			bid := "status-server"
			connectionType := "connect"
			scopes := []string{"stats"}
			topic := "stats"
			token, err := token.New(iat, nbf, exp, scopes, aud, bid, connectionType, config.SecretJump, topic)
			log.Tracef("token: [%s]", token)
			if err != nil {
				log.WithField("error", err.Error()).Error("Jump stats token generation failed")
				time.Sleep(5 * time.Second) //rate-limit retries if there is a long standing issue
				break
			}
			ctxStats, cancel := context.WithTimeout(ctx, time.Second*time.Duration(config.ReconnectJumpEvery))
			to := aud + "/api/v1/" + connectionType + "/" + topic
			log.Tracef("to: [%s]", to)
			j.Connect(ctxStats, to, token)
			cancel() // call to save leaking, even though cancelled before getting to here
		}
	}
}

func (s *Status) processRelay(ctx context.Context, r *rc.Status) {

	for {
		select {
		case <-ctx.Done():
			return
		case reports, ok := <-r.Status:
			if !ok {
				log.Error("relay status channel not ok, stopping processing status")
				return
			}

			s.updateFromRelay(reports)

		}
	}
}

func (s *Status) processJump(ctx context.Context, j *jc.Status) {

	for {
		select {
		case <-ctx.Done():
			return
		case reports, ok := <-j.Status:
			if !ok {
				log.Error("jump status channel not ok, stopping processing status")
				return
			}

			s.updateFromJump(reports)

		}

	}

}

func (s *Status) updateFromJump(reports []jc.Report) {
	s.Lock()
	defer s.Unlock()
	now := time.Now()
	for _, r := range reports {

		// skip non-hosts
		if !slices.Contains(r.Scopes, "host") {
			continue
		}

		// initialise experiment's entry if not yet present
		if _, ok := s.Experiments[r.Topic]; !ok {
			s.Experiments[r.Topic] = NewReport()
		}

		expt := s.Experiments[r.Topic]
		expt.JumpReport = r
		expt.LastCheckedJump = now
		expt.JumpOK = true //assume that a live connection is equivalent to a healthy one

		s.Experiments[r.Topic] = expt
	}

	s.UpdateHealth()

}

func (s *Status) updateFromRelay(reports []rc.Report) {
	s.Lock()
	defer s.Unlock()

	now := time.Now()

	for _, r := range reports {

		id, err := getID(r.Topic)
		if err != nil {
			continue
		}
		// skip non-expts
		if !slices.Contains(r.Scopes, "expt") {
			continue
		}

		// initialise experiment's entry if not yet present
		if _, ok := s.Experiments[id]; !ok {
			s.Experiments[id] = NewReport()
		}

		stream, err := getStream(r.Topic)
		if err != nil {
			continue
		}

		expt := s.Experiments[id]

		expt.StreamReports[stream] = r

		expt.StreamOK[stream] = true

		if r.Stats.Tx.Last > s.Config.HealthLast {
			expt.StreamOK[stream] = false
		}

		if r.Stats.Tx.Never == true {
			expt.StreamOK[stream] = false
		}

		expt.LastCheckedStreams = now

		s.Experiments[id] = expt

	}

	s.UpdateHealth()
}

func getID(str string) (string, error) {

	id := regexp.MustCompile(`([[:alnum:]].*)-st-[[:alnum:]].*`)
	result := id.FindStringSubmatch(str)
	if len(result) == 2 {
		return result[1], nil
	}

	return "", errors.New("not found")
}

func getStream(str string) (string, error) {

	stream := regexp.MustCompile(`[[:alnum:]].*-st-([[:alnum:]].*)`)
	result := stream.FindStringSubmatch(str)
	if len(result) == 2 {
		return result[1], nil
	}

	return "", errors.New("not found")

}

func (s *Status) UpdateHealth() {

	alerts := make(map[string]bool)

	hm := make(map[string]HealthyIssues)

	now := time.Now()

	for k, v := range s.Experiments {

		// jump must be ok
		h := true
		issues := []string{}

		if !v.JumpOK {
			h = false
			issues = append(issues, "jump present but not ok")
		}

		// jump must have been recently checked
		if !(v.LastCheckedJump.Add(s.Config.HealthLast)).After(now) {
			h = false
			issues = append(issues, "jump missing")
		}

		// all required streams must be present
		for stream, _ := range v.StreamRequired {
			if _, ok := v.StreamOK[stream]; !ok {
				h = false
				issues = append(issues, "required stream missing ("+stream+")")
			}
		}

		// the streams must have been recently checked
		if !(v.LastCheckedStreams.Add(s.Config.HealthLast)).After(now) {
			h = false
			issues = append(issues, "no current streams")
		}

		// all streams must be healthy (even if not required)
		for stream, sok := range v.StreamOK {
			if !sok {
				h = false
				issues = append(issues, "stream unhealthy ("+stream+")")
			}
		}

		hm[k] = HealthyIssues{
			Healthy: h,
			Issues:  issues,
		}

	}

	// update Experiments map entires with their overall health
	// make an alert map of experiments that have just become healthy or unhealthy
	// alert map

	expt := s.Experiments
	for k, v := range hm {
		r := expt[k]

		// skip adding health events until experiment has had a chance to
		// get complete information from jump and relay
		// typically set HealthStartup to 3x the slowest reporting period
		// from jump / relay
		if now.Before(r.FirstChecked.Add(s.Config.HealthStartup)) {
			continue
		}

		// add event if health has changed, or if no event previously registered
		if r.Healthy != v.Healthy || len(r.HealthEvents) == 0 {
			he := HealthEvent{
				Healthy:  v.Healthy,
				When:     now,
				JumpOK:   r.JumpOK,
				StreamOK: r.StreamOK,
				Issues:   v.Issues,
			}

			r.HealthEvents = append(r.HealthEvents, he)

			if len(r.HealthEvents) > s.Config.HealthEvents {
				r.HealthEvents = r.HealthEvents[len(r.HealthEvents)-s.Config.HealthEvents:]
			}

			alerts[k] = v.Healthy
		}

		r.Healthy = v.Healthy

		expt[k] = r
	}
	s.Experiments = expt

	// skip email if no alerts
	if len(alerts) == 0 {
		log.Trace("No email needed")
		return
	}

	count := strconv.Itoa(len(alerts))

	auth := smtp.PlainAuth("", s.Config.EmailFrom, s.Config.EmailPassword, s.Config.EmailHost)

	toHeader := strings.Join(s.Config.EmailTo, ",")
	ccHeader := strings.Join(s.Config.EmailCc, ",")
	receivers := append(s.Config.EmailTo, s.Config.EmailCc...)
	// The msg parameter should be an RFC 822-style email with headers first, a blank line, and then the message body. The lines of msg should be CRLF terminated. The msg headers should usually include fields such as "From", "To", "Subject", and "Cc".

	msg := "To: " + toHeader + "\r\n" +
		"Cc: " + ccHeader + "\r\n" +
		"Subject: " + s.Config.EmailSubject + " " + count + " new health events \r\n" +
		"\r\n" +
		"There are " + count + " new health events: \r\n"

	for k, v := range alerts {
		if v {
			msg += k + " OK\r\n"
		} else {
			he := s.Experiments[k].HealthEvents
			issues := "[unknown]"
			if len(he) > 0 {
				issues = "[" + strings.Join(he[len(he)-1].Issues, ",") + "]"
			}
			msg += k + " ** issue ** " + issues + "\r\n"
		}

	}

	msg += "\r\n\r\n For the latest complete status information, please go to " + s.Config.EmailLink + "\r\n"

	log.Errorf(msg)
	// EmailTo must be []string
	hostPort := s.Config.EmailHost + ":" + strconv.Itoa(s.Config.EmailPort)
	err := smtp.SendMail(hostPort, auth, s.Config.EmailFrom, receivers, []byte(msg))

	if err != nil {

		log.Error(err)

	}

}
