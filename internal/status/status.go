package status

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"net/smtp"

	bc "github.com/practable/book/pkg/resource"
	jc "github.com/practable/jump/pkg/status"
	rc "github.com/practable/relay/pkg/status"
	"github.com/practable/relay/pkg/token"
	"github.com/practable/status/internal/config"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

//ro "github.com/practable/status/internal/relay/operations"
//rm "github.com/practable/status/internal/relay/models"

func NewReport(s *config.Status) config.Report {

	return config.Report{
		FirstChecked:   s.Now(),
		Healthy:        false,
		HealthEvents:   []config.HealthEvent{},
		JumpOK:         false,
		JumpReport:     jc.Report{},
		StreamOK:       make(map[string]bool),
		StreamReports:  make(map[string]rc.Report),
		StreamRequired: make(map[string]bool),
	}

}

// Run handles connecting to book, jump, relay and updating status
// jc.Status and rc.Status clients are passed in to allow mocking during testing
func Run(ctx context.Context, j *jc.Status, r *rc.Status, s *config.Status) {

	go connectRelay(ctx, s, r)
	go connectJump(ctx, s, j)

	go processRelay(ctx, s, r)
	go processJump(ctx, s, j)
	go logHealth(ctx, s) //necessary to detect the case in which jump and relay both stop
	go connectBook(ctx, s)

	<-ctx.Done()
	log.Trace("Status done")

}

func connectRelay(ctx context.Context, s *config.Status, r *rc.Status) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// make token for relay stats connection for duration config.ReconnectRelayEvery
			iat := s.Now()
			nbf := s.Now()
			exp := s.Now().Add(s.Config.ReconnectRelayEvery)
			log.WithFields(log.Fields{"iat": iat, "nbf": nbf, "exp": exp}).Trace("Token times")
			aud := s.Config.SchemeRelay + "://" + path.Join(s.Config.HostRelay, s.Config.BasepathRelay)
			bid := "status-server"
			connectionType := "session"
			scopes := []string{"read"}
			topic := "stats"
			token, err := token.New(iat, nbf, exp, scopes, aud, bid, connectionType, s.Config.SecretRelay, topic)
			log.Tracef("token: [%s]", token)
			if err != nil {
				log.WithField("error", err.Error()).Error("Relay stats token generation failed")
				time.Sleep(5 * time.Second) //rate-limit retries if there is a long standing issue
				break
			}
			ctxStats, cancel := context.WithTimeout(ctx, s.Config.ReconnectRelayEvery)
			to := aud + "/" + connectionType + "/" + topic
			log.Tracef("to: [%s]", to)
			r.Connect(ctxStats, to, token)
			cancel() // call to save leaking, even though cancelled before getting to here
		}
	}
}

func connectJump(ctx context.Context, s *config.Status, j *jc.Status) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// make token for relay stats connection for duration config.ReconnectRelayEvery
			iat := s.Now()
			nbf := s.Now()
			exp := s.Now().Add(s.Config.ReconnectJumpEvery)
			log.WithFields(log.Fields{"iat": iat, "nbf": nbf, "exp": exp}).Trace("Token times")
			aud := s.Config.SchemeJump + "://" + path.Join(s.Config.HostJump, s.Config.BasepathJump)
			bid := "status-server"
			connectionType := "connect"
			scopes := []string{"stats"}
			topic := "stats"
			token, err := token.New(iat, nbf, exp, scopes, aud, bid, connectionType, s.Config.SecretJump, topic)
			log.Tracef("token: [%s]", token)
			if err != nil {
				log.WithField("error", err.Error()).Error("Jump stats token generation failed")
				time.Sleep(5 * time.Second) //rate-limit retries if there is a long standing issue
				break
			}
			ctxStats, cancel := context.WithTimeout(ctx, s.Config.ReconnectJumpEvery)
			to := aud + "/api/v1/" + connectionType + "/" + topic
			log.Tracef("to: [%s]", to)
			j.Connect(ctxStats, to, token)
			cancel() // call to save leaking, even though cancelled before getting to here
		}
	}
}

func connectBook(ctx context.Context, s *config.Status) {

	// basepath has slightly different interpretations between our config
	// and how it is used in bc.Config. TODO check this works.
	c := bc.Config{
		BasePath: "/api/v1",
		Host:     s.Config.HostBook + s.Config.BasepathBook,
		Scheme:   s.Config.SchemeBook,
		Timeout:  time.Duration(5 * time.Second),
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(s.Config.QueryBookEvery):
			// make token with updated times
			audience := s.Config.SchemeBook + "://" + s.Config.HostBook + s.Config.BasepathBook
			subject := "status-server"
			secret := s.Config.SecretBook
			scopes := []string{"booking:admin"}

			iat := s.Now()
			nbf := s.Now().Add(-1 * time.Second)
			exp := s.Now().Add(s.Config.QueryBookEvery)

			tk, err := bc.NewToken(audience, subject, secret, scopes, iat, nbf, exp)

			c.Token = tk

			if err != nil {
				log.Errorf("connectBook error making token was %s", err.Error())
				continue
			}

			resources, err := c.GetResources()

			if err != nil {
				log.Errorf("connectBook error getting resources was %s", err.Error())
				continue
			}

			for _, v := range resources {
				r := v
				streams := make(map[string]bool)
				for _, stream := range r.Streams {
					fullStream := r.TopicStub + "-" + stream //this convention comes from the way manifest handle stream names
					streams[fullStream] = true
				}
				ex := s.Experiments[r.TopicStub]
				ex.StreamRequired = streams
				ex.ResourceName = r.Name
				ex.LastFoundInManifest = s.Now()
				s.Experiments[r.TopicStub] = ex
			}

			updateHealth(s) //force an update in case connections to relay and jump are down
		}

	}
}

func logHealth(ctx context.Context, s *config.Status) {

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(s.Config.HealthLogEvery):
			updateHealth(s) //force an update in case connections to relay and jump are down
			status, err := json.Marshal(s.Experiments)
			if err != nil {
				log.Errorf("HealthLogEvery marshal error %s", err.Error())
				continue
			}
			log.WithField("status", string(status)).Infof("Current Health")

		}

	}

}

func processRelay(ctx context.Context, s *config.Status, r *rc.Status) {

	for {
		select {
		case <-ctx.Done():
			return
		case reports, ok := <-r.Status:
			if !ok {
				log.Error("relay status channel not ok, stopping processing status")
				return
			}
			log.Infof("Relay status update received of size %d", len(reports))
			updateFromRelay(s, reports)

		}
	}
}

func processJump(ctx context.Context, s *config.Status, j *jc.Status) {

	for {
		select {
		case <-ctx.Done():
			return
		case reports, ok := <-j.Status:
			if !ok {
				log.Error("jump status channel not ok, stopping processing status")
				return
			}

			updateFromJump(s, reports)

		}

	}

}

func updateFromJump(s *config.Status, reports []jc.Report) {
	s.Lock()
	defer s.Unlock()
	now := s.Now()
	for _, r := range reports {

		// skip non-hosts
		if !slices.Contains(r.Scopes, "host") {
			continue
		}

		// initialise experiment's entry if not yet present
		if _, ok := s.Experiments[r.Topic]; !ok {
			s.Experiments[r.Topic] = NewReport(s) //pass status to get time
		}

		expt := s.Experiments[r.Topic]
		expt.JumpReport = r
		expt.LastCheckedJump = now
		expt.JumpOK = true //assume that a live connection is equivalent to a healthy one

		s.Experiments[r.Topic] = expt
	}

	updateHealth(s)

}

func updateFromRelay(s *config.Status, reports []rc.Report) {
	s.Lock()
	defer s.Unlock()

	now := s.Now()

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
			s.Experiments[id] = NewReport(s) //pass status to get time
		}

		stream := r.Topic

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

	updateHealth(s)
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

func updateHealth(s *config.Status) {

	alerts := make(map[string]bool)

	hm := make(map[string]config.HealthyIssues)

	now := s.Now()

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

		hm[k] = config.HealthyIssues{
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
			he := config.HealthEvent{
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

	emailAuthType := strings.TrimSpace(strings.ToLower(s.Config.EmailAuthType))

	plainAuth := smtp.PlainAuth("", s.Config.EmailFrom, s.Config.EmailPassword, s.Config.EmailHost)

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

	msg += "\r\n\r\n All new and existing health issues:\r\n"

	for k, v := range s.Experiments {
		if !v.Healthy {
			he := v.HealthEvents
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

	log.WithFields(log.Fields{"s.Config.EmailHost": s.Config.EmailHost, "s.Config.EmailPort": s.Config.EmailPort, "hostPort": hostPort, "s.Config.EmailFrom": s.Config.EmailFrom, "receivers": receivers}).Debug("Email host")

	var err error

	switch emailAuthType {
	case "plain":
		err = smtp.SendMail(hostPort, plainAuth, s.Config.EmailFrom, receivers, []byte(msg))
	case "none":
		err = smtp.SendMail(hostPort, nil, s.Config.EmailFrom, receivers, []byte(msg))
	default:
		err = fmt.Errorf("Email auth type unknown (%s), should be [plain, none]", s.Config.EmailAuthType)
	}

	if err != nil {

		log.Error(err)

	}

}
