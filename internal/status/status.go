package status

import (
	"context"
	"errors"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	jc "github.com/practable/jump/pkg/status"
	rc "github.com/practable/relay/pkg/status"
	"github.com/practable/relay/pkg/token"
	log "github.com/sirupsen/logrus"
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
	IPAddress  map[string]string
	Experiment map[string]Report
}

// Report represents the status
type Report struct {
	Healthy bool
	Streams map[string]bool
	Reports map[string]rc.Report
}

// New returns an Status with initialised maps
func New() *Status {
	return &Status{
		&sync.RWMutex{},
		make(map[string]string),
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

		if strings.Contains(r.Topic, "/") {
			continue // only host connections have no slash in the topic
		}

		s.IPAddress[r.Topic] = strings.TrimSpace(r.RemoteAddr)

	}
}

func (s *Status) updateFromRelay(reports []rc.Report, config Config) {
	s.Lock()
	defer s.Unlock()

	for _, r := range reports {

		id, err := getID(r.Topic)
		if err != nil {
			continue
		}

		// check IP address is in map of experiments

		if _, ok := s.IPAddress[id]; !ok {
			continue // can't process if don't have an IP to check
		}

		if s.IPAddress[id] != strings.TrimSpace(r.RemoteAddr) {
			continue // no match means not an experiment, must be a user connection
		}

		if _, ok := s.Experiment[id]; !ok {

			s.Experiment[id] = Report{
				false,
				make(map[string]bool),
				make(map[string]rc.Report),
			}
		}

		stream, err := getStream(r.Topic)
		if err != nil {
			continue
		}

		exp := s.Experiment[id]

		exp.Reports[stream] = r

		exp.Streams[stream] = true

		if r.Stats.Tx.Last > config.HealthyDuration {
			exp.Streams[stream] = false
		}

		if r.Stats.Tx.Never == true {
			exp.Streams[stream] = false
		}

		//TODO update to compare to the booking manifest's required streams
		// this check can give false positive if a stream is missing entirely
		// e.g. if experiment is misconfigured
		exp.Healthy = true

		for _, healthy := range exp.Streams {
			if !healthy {
				exp.Healthy = false
			}
		}

		s.Experiment[id] = exp

	}
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
