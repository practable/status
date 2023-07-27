package status

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"sync"
	"time"

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
	EmailPassword       string
	EmailPort           int
	EmailTo             string
	HealthyDuration     time.Duration
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
	Experiments map[string]Report
}

// Report represents the status
type Report struct {
	Healthy        bool
	JumpOK         bool
	JumpReport     jc.Report
	StreamOK       map[string]bool
	StreamReports  map[string]rc.Report
	StreamRequired map[string]bool
}

func NewReport() Report {
	return Report{
		Healthy:        false,
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
		make(map[string]Report),
	}

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
	s := New()
	r := rc.New()
	j := jc.New()

	go connectRelay(ctx, config, r)
	go connectJump(ctx, config, j)

	go processRelay(ctx, s, r, config)
	go processJump(ctx, s, j)

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

func processRelay(ctx context.Context, s *Status, r *rc.Status, config Config) {

	for {
		select {
		case <-ctx.Done():
			return
		case status, ok := <-r.Status:
			if !ok {
				log.Error("relay status channel not ok, stopping processing status")
				return
			}

			s.updateFromRelay(status, config)

		}
	}
}

func processJump(ctx context.Context, s *Status, j *jc.Status) {

	for {
		select {
		case <-ctx.Done():
			return
		case status, ok := <-j.Status:
			if !ok {
				log.Error("jump status channel not ok, stopping processing status")
				return
			}

			s.updateFromJump(status)

		}

	}

}

func (s *Status) updateFromJump(reports []jc.Report) {
	s.Lock()
	defer s.Unlock()

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
		expt.JumpOK = true //assume that a live connection is equivalent to a healthy one

		s.Experiments[r.Topic] = expt
	}

	s.UpdateHealth()

}

func (s *Status) updateFromRelay(reports []rc.Report, config Config) {
	s.Lock()
	defer s.Unlock()

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

		exp := s.Experiments[id]

		exp.StreamReports[stream] = r

		exp.StreamOK[stream] = true

		if r.Stats.Tx.Last > config.HealthyDuration {
			exp.StreamOK[stream] = false
		}

		if r.Stats.Tx.Never == true {
			exp.StreamOK[stream] = false
		}

		s.Experiments[id] = exp

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

	hm := make(map[string]bool)

	for k, v := range s.Experiments {

		// jump must be ok
		h := v.JumpOK

		// all required streams must be present
		for stream, _ := range v.StreamRequired {
			if _, ok := v.StreamOK[stream]; !ok {
				h = false
			}
		}

		// all streams must be healthy (even if not required)
		for _, sok := range v.StreamOK {
			h = h && sok
		}

		hm[k] = h

	}

	// update Experimentss map entires with their overall health
	expt := s.Experiments
	for k, v := range hm {
		r := expt[k]
		r.Healthy = v
		expt[k] = r
	}
	s.Experiments = expt

}
