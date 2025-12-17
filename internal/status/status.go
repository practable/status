package status

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"path"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	bc "github.com/practable/book/pkg/resource"
	"github.com/practable/relay/pkg/token"
	rt "github.com/practable/relay/pkg/token"
	"github.com/practable/status/internal/config"
	log "github.com/sirupsen/logrus"
)

type RelayReport struct {
	CanRead     bool     `json:"can_read"`
	CanWrite    bool     `json:"can_write"`
	ConnectedAt string   `json:"connected_at"`
	ExpiresAt   string   `json:"expires_at"`
	RemoteAddr  string   `json:"remote_addr"`
	Scopes      []string `json:"scopes"`
	Topic       string   `json:"topic"`
	UserAgent   string   `json:"user_agent"`
}

type StatusUpdate struct {
	UpdateFunc func(*config.Status)
}

func startStatusUpdater(s *config.Status) chan StatusUpdate {
	updateChan := make(chan StatusUpdate)
	go func() {
		for update := range updateChan {
			s.Lock()
			update.UpdateFunc(s)
			s.Unlock()
		}
	}()
	return updateChan
}

// FindStreamEvents compares two maps of experiment streams
// and returns a list of events for streams that have appeared or disappeared
func FindStreamEvents(current, last map[string]map[string]bool) (map[string]map[string]bool, error) {

	events := make(map[string]map[string]bool)

	// check for streams we had before, that aren't present now
	for expt, streams := range last {
		for stream, wasPresent := range streams {

			//fmt.Printf("checking expt %s stream %s was present: %s\n", expt, stream, strconv.FormatBool(wasPresent))

			isPresent, ok := current[expt][stream]

			if ok {
				if wasPresent == isPresent {
					continue // same, no event
				}
			}

			if wasPresent { //gone missing

				if val, ok := events[expt]; !ok {
					ev := make(map[string]bool)
					ev[stream] = false
					events[expt] = ev

				} else {
					val[stream] = false
					events[expt] = val
				}
			}

			// else still missing, no event
		}
	}

	// check for new streams
	for expt, streams := range current {
		for stream, isPresent := range streams {
			wasPresent, ok := last[expt][stream]

			if ok {
				if wasPresent == isPresent {
					continue // same, no event
				}
			}

			if isPresent {
				// either not present in the old map, or false in the old map, either way, it's new
				if val, ok := events[expt]; !ok {
					ev := make(map[string]bool)
					ev[stream] = true
					events[expt] = ev

				} else {
					val[stream] = true
					events[expt] = val
				}
			}
		}
	}

	return events, nil

}

func CheckStreams(ctx context.Context, c config.Config) (map[string]map[string]bool, error) {

	// only connected streams are listed in the relay status info
	// so consider a report that is present with scope "expt" to mean that stream is
	// currently being supplied by an experiment
	// we would need to check the quality of the stream in a separate quality check
	// outside the scope of this code

	resources, err := BookGetResources(ctx, c)

	if err != nil {
		return nil, errors.New("error getting book resources: " + err.Error())
	}

	// iterate over resources to get list of expected streams
	// for each resource, we will make a map of the stream names
	// to set us up to check them off from the relay report
	// anything we expect that is not present, remains "false"
	checkedStreams := make(map[string]map[string]bool)

	for _, r := range resources {
		name := r.TopicStub //avoid repeated field access
		streams := make(map[string]bool)
		for _, s := range r.Streams {
			topic := name + "-" + s //this convention comes from the way manifest handle stream names
			streams[topic] = false  //not checked it yet, so set false
		}
		checkedStreams[name] = streams
	}

	// We don't need to make a separate map for unexpected streams
	// not in the manifest - we can simply add them to the list and
	// report them as we find them in the relay report

	status, err := RelayGetStatus(ctx, c)

	if err != nil {
		return nil, errors.New("error getting relay status: " + err.Error())
	}

	for _, r := range status {
		// check for expt scope (we don't track non-experiment streams)
		if !slices.Contains(r.Scopes, "expt") {
			continue //skip non-experiment streams
		}
		s := r.Topic // complete stream name
		// find which experiment this stream belongs to
		name, err := getID(s) //assumes topic is of form <topicstub>-<streamname>
		if err != nil {
			log.Errorf("CheckStreams: error getting experiment name from stream %s: %s", s, err.Error())
			continue
		}

		if _, ok := checkedStreams[name]; !ok {
			// stream not from an experiment listed in manifest
			// add a new entry for streams from this experiment
			checkedStreams[name] = make(map[string]bool)
		}

		// extract the map of streams for this experiment, so we can edit it
		streamsMap := checkedStreams[name]
		streamsMap[s] = true //mark as connnected, because found in relay report
		checkedStreams[name] = streamsMap
	}

	return checkedStreams, nil
}

// Func BookGetResources connects to the book server and gets the list of resources
func BookGetResources(ctx context.Context, c config.Config) ([]bc.About, error) {

	b := bc.Config{
		BasePath: "/api/v1",
		Host:     c.HostBook + c.BasepathBook,
		Scheme:   c.SchemeBook,
		Timeout:  time.Duration(5 * time.Second),
	}

	// make token with updated times
	audience := c.SchemeBook + "://" + c.HostBook + c.BasepathBook
	subject := "status-server"
	secret := c.SecretBook
	scopes := []string{"booking:admin"}

	now := time.Now()

	iat := now
	nbf := now.Add(-1 * time.Second) // avoid clock skew issues
	exp := now.Add(30 * time.Second) // short lived token for single API request

	tk, err := bc.NewToken(audience, subject, secret, scopes, iat, nbf, exp)

	b.Token = tk

	log.WithFields(log.Fields{"basepath": b.BasePath, "host": b.Host, "scheme": b.Scheme, "token": b.Token}).Debug("book connection details")

	if err != nil {
		log.Errorf("connectBook error making token was %s", err.Error())
		return nil, err
	}

	return b.GetResources()

}

func RelayGetStatus(ctx context.Context, c config.Config) ([]RelayReport, error) {

	// create token
	now := time.Now()

	iat := now
	nbf := now.Add(-1 * time.Second) // avoid clock skew issues
	exp := now.Add(30 * time.Second) // short lived token for single API request

	scopes := []string{"relay:stats"}

	audience := c.SchemeRelay + "://" + c.HostRelay + c.BasepathRelay

	// these ones don't matter much for relay stats, but populating anyway
	bid := "status-server"
	connectionType := "session"
	topic := "stats"

	bearer, err := rt.New(iat, nbf, exp, scopes, audience, bid, connectionType, c.SecretRelay, topic)

	if err != nil {
		return nil, errors.New("error making token: " + err.Error())
	}

	// GET from API
	client := &http.Client{}
	url := c.SchemeRelay + "://" + c.HostRelay + c.BasepathRelay + "/status"
	log.Debugf("getStatus: url is %s", url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Errorf("getStatus: new request error was %s", err.Error())
		return nil, err
	}
	req.Header.Add("Authorization", bearer)

	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("getStatus: do request error was %s", err.Error())
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		log.Errorf("getStatus: Status code was %d", resp.StatusCode)
		return nil, fmt.Errorf("status code was %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("GetStatus: io.ReadAll error is %s", err.Error())
		return nil, err
	}

	results := []RelayReport{}
	log.Tracef("GetStatus:  body is %s", string(body))

	err = json.Unmarshal(body, &results)

	if err != nil {
		log.Errorf("GetStatus: unmarshal error is %s", err.Error())
		return nil, err
	}
	return results, nil
}

func Run(ctx context.Context, s *config.Status) {

	//first check
	err := updateHealth(ctx, s, true)

	if err != nil {
		log.Errorf("logHealth: error initialising status with the first health check: %s", err.Error())
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(s.Config.HealthLogEvery):
			err := updateHealth(ctx, s, false)
			if err != nil {
				log.Errorf("logHealth: error updating health: %s", err.Error())
			}
		}

	}
}

// make the experiment reports needed by the API
func UpdateExperiments(ctx context.Context, s *config.Status) error {

}

func UpdateStreams(ctx context.Context, s *config.Status, firstTime bool) error {

	s.Unlock()
	defer s.Lock()

	check, err := CheckStreams(ctx, s.Config)

	if err != nil {
		log.Errorf("updateHealth: error checking streams: %s", err.Error())
		return
	}

	events := make(map[string]map[string]bool)

	if !firstTime {

		events, err := FindStreamEvents(check, s.LastCheck)

		if err != nil {
			log.Errorf("updateHealth: error finding stream events: %s", err.Error())
			return
		}
	}

	s.LastCheck = check

	// use events to populate experiment reports

	return

}

// this function is called by the status updater to update the health of the system
func updateHealth(updateChan chan StatusUpdate) {
	updateChan <- StatusUpdate{
		UpdateFunc: func(s *config.Status) {

			alerts := make(map[string]bool)

			now := s.Now() //keep same for all entries from this set of reports

			systemAlerts := []string{}

			// check for staleness of reports
			jumpStale := make(map[string]bool)
			streamsStale := make(map[string]bool)

			//TODO
			//
			//	change the philosophy from permanent connection, to connnection-on-demand
			//	therefore, a stream is considered healthy if the current time
			//	is between the connectedAt and expiresAt time
			//
			//	we'll do additional checks on data in the streams if needed, as a separate
			//	task, not for this minimal update to get working with new relay again
			//	(new relay has no stats)
			//
			//	There's very little point in trying to keep tabs on stats if we want a quick
			//	check on whether a stream has connected or not, e.g. new immediate-use booking
			//	has been made, and we've asked the experiment to connect - has it?
			//	This should also reduce some of the false alerts with the current system

			for k, v := range s.Experiments {

				js := (v.LastCheckedJump.Add(s.Config.HealthLastChecked)).Before(now)
				ss := (v.LastCheckedStreams.Add(s.Config.HealthLastChecked)).Before(now)
				jumpStale[k] = js
				streamsStale[k] = ss

				if ss {
					log.WithFields(log.Fields{"now": now, "lastCheckedStreams": v.LastCheckedStreams, "last+health": v.LastCheckedStreams.Add(s.Config.HealthLastChecked), "stale": ss, "topic": k}).Debug("stale stream")
				}
			}

			jsc := 0
			for _, v := range jumpStale {
				if v {
					jsc += 1
				}
			}
			ssc := 0
			for _, v := range streamsStale {
				if v {
					ssc += 1
				}
			}

			log.WithFields(log.Fields{"jump": jumpStale, "streams": streamsStale}).Debugf("%d/%d jump stale, %d/%d streams stale", jsc, len(jumpStale), ssc, len(streamsStale))

			if jsc == len(jumpStale) {
				msg := "jump has no fresh reports so may be down"
				log.Error(msg)
				systemAlerts = append(systemAlerts, msg)
			}

			if ssc == len(streamsStale) {
				msg := "relay has no fresh reports so may be down"
				log.Error(msg)
				systemAlerts = append(systemAlerts, msg)
			}

			// check for inactive streams

			streamsActive := make(map[string]map[string]bool)

			for k, v := range s.Experiments {

				sm := v.StreamOK

				for sk := range v.StreamOK {
					recent := (v.StreamReports[sk].Stats.Tx.Last < s.Config.HealthLastActive)
					never := v.StreamReports[sk].Stats.Tx.Never
					active := recent && !never
					if !active {
						log.WithFields(log.Fields{"stream": sk, "never": never, "recent": recent, "last": v.StreamReports[sk].Stats.Tx.Last}).Errorf("inactive stream")
					} else {
						log.WithFields(log.Fields{"stream": sk, "never": never, "recent": recent, "last": v.StreamReports[sk].Stats.Tx.Last}).Debugf("active stream")
					}
					sm[sk] = active
				}

				streamsActive[k] = sm

			}

			// update experiments that are showing stale checks
			for k, v := range jumpStale {

				expt := s.Experiments[k]

				expt.JumpOK = !v //v is inverted

				s.Experiments[k] = expt

			}

			for k, v := range streamsActive {

				expt := s.Experiments[k]

				for sk, vv := range v {
					expt.StreamOK[sk] = vv
				}
				s.Experiments[k] = expt
			}

			for k, v := range streamsStale {

				expt := s.Experiments[k]

				es := expt.StreamOK

				for sk := range es {
					expt.StreamOK[sk] = expt.StreamOK[sk] && !v // must be active and NOT stale (!v)
				}

				s.Experiments[k] = expt
			}

			// check for issues

			// health events map
			hm := make(map[string]config.HealthyIssues)

			// available map
			am := make(map[string]bool)

			for k, v := range s.Experiments {

				// jump must be ok
				h := true
				a := true
				issues := []string{}

				if !v.JumpOK {
					h = false //no jump is unhealthy BUT
					// don't change availability based on jump, leave a true for now
					issues = append(issues, "missing jump")
				}

				// all required streams must be present
				for stream := range v.StreamRequired {
					if _, ok := v.StreamOK[stream]; !ok {
						h = false
						a = false
						issues = append(issues, "missing required "+stream)
					}
				}

				// no need to check for stale streams explicitly - already done by the stale checks which
				// change the maps

				// all streams must be healthy (even if not required)
				for stream, sok := range v.StreamOK {
					if !sok {
						h = false
						// don't change the availability, if all required streams are good, it can be available for booking
						issues = append(issues, "unhealthy "+stream)
					}
				}

				hm[k] = config.HealthyIssues{
					Healthy:     h,
					JumpHealthy: v.JumpOK,
					Issues:      issues,
				}

				// update availability map
				am[k] = a

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

				// add event if health has changed, or if no event previously registered, or if jump has changed
				if r.Healthy != v.Healthy || len(r.HealthEvents) == 0 || r.JumpHealthy != v.JumpHealthy {
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
				r.JumpHealthy = v.JumpHealthy
				expt[k] = r
			}

			// set resource availability where it has changed, according to availability map am

			c := bc.Config{
				BasePath: "/api/v1",
				Host:     s.Config.HostBook + s.Config.BasepathBook,
				Scheme:   s.Config.SchemeBook,
				Timeout:  time.Duration(5 * time.Second), //local server, should respond fast
			}

			audience := s.Config.SchemeBook + "://" + s.Config.HostBook + s.Config.BasepathBook
			subject := "status-server"
			secret := s.Config.SecretBook
			scopes := []string{"booking:admin"}

			iat := s.Now()
			nbf := s.Now().Add(-1 * time.Second)
			exp := s.Now().Add(time.Hour) // good for > 500 requests at 5sec timeout

			tk, err := bc.NewToken(audience, subject, secret, scopes, iat, nbf, exp)

			c.Token = tk

			if err != nil {
				log.Errorf("no changes in availability possible because error making book token was %s", err.Error())
			} else { // no point in trying to set availability unless token is ok

				for k, v := range am {

					r := expt[k]

					if now.Before(r.FirstChecked.Add(s.Config.HealthStartup)) {
						continue // ignore for now in case this is a status server restart - we'll be able to take anything unhealthy offline within config.HealthStartup of starting the status server
					}

					if r.Available != v {

						if r.ResourceName == "" {
							log.Infof("changeAvailability skipping setting availability for %s because no ResourceName (probably not in manifest)", k)
							continue
						}

						//change on book server
						reason := "status server: " + s.Now().String()
						err := c.SetResourceAvailability(r.ResourceName, v, reason)

						if err != nil {
							log.Errorf("changeAvailability error setting availability for %s(%s) was %s", k, r.ResourceName, err.Error())
							continue
						}

						log.WithFields(log.Fields{"experiment": k, "resourceName": r.ResourceName, "available": v, "reason": reason}).Infof("changeAvailability set availability of %s to %t)", k, v)

						// check change, and update status
						status, err := c.GetResourceAvailability(r.ResourceName)
						if err != nil {
							log.Errorf("changeAvailability error getting availability for %s(%s) was %s", k, r.ResourceName, err.Error())
							continue
						}

						r.Available = status.Available

						if r.Available != v {
							log.Errorf("changeAvailability error setting correct availability have %v want %v", r.Available, v)
						}

					}

					expt[k] = r //record the availability of the resource
				}

			}

			s.Experiments = expt

			// skip email if no alerts
			if len(alerts) == 0 {
				log.Trace("No email needed")
				return
			}

			emailAuthType := strings.TrimSpace(strings.ToLower(s.Config.EmailAuthType))

			plainAuth := smtp.PlainAuth("", s.Config.EmailFrom, s.Config.EmailPassword, s.Config.EmailHost)

			msg := EmailBody(s, alerts, systemAlerts)

			log.Error(msg)
			// EmailTo must be []string

			hostPort := s.Config.EmailHost + ":" + strconv.Itoa(s.Config.EmailPort)

			receivers := append(s.Config.EmailTo, s.Config.EmailCc...)

			log.WithFields(log.Fields{"s.Config.EmailHost": s.Config.EmailHost, "s.Config.EmailPort": s.Config.EmailPort, "hostPort": hostPort, "s.Config.EmailFrom": s.Config.EmailFrom, "receivers": receivers}).Debug("Email host")

			switch emailAuthType {
			case "plain":
				err = smtp.SendMail(hostPort, plainAuth, s.Config.EmailFrom, receivers, []byte(msg))
			case "none":
				err = smtp.SendMail(hostPort, nil, s.Config.EmailFrom, receivers, []byte(msg))
			default:
				err = fmt.Errorf("email auth type unknown (%s), should be [plain, none]", s.Config.EmailAuthType)
			}

			if err != nil {

				log.Error(err)

			}
		},
	}
}

func NewReport(now time.Time) config.Report {

	return config.Report{
		FirstChecked:   now,
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

	updateChan := startStatusUpdater(s)

	go connectRelay(ctx, s, r)
	go connectJump(ctx, s, j)

	go processRelay(ctx, r, updateChan)
	go processJump(ctx, j, updateChan)
	go logHealth(ctx, s, updateChan) //necessary to detect the case in which jump and relay both stop
	go connectBook(ctx, s, updateChan)

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
			s.Lock()
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
			s.Unlock() //release now avoid waiting 5 seconds in error handler whilst holding lock

			if err != nil {
				log.WithField("error", err.Error()).Error("Relay stats token generation failed")
				time.Sleep(5 * time.Second) //rate-limit retries if there is a long standing issue
				break
			}
			s.Lock()
			ctxStats, cancel := context.WithTimeout(ctx, s.Config.ReconnectRelayEvery)
			s.Unlock()
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

func logHealth(ctx context.Context, s *config.Status, updateChan chan StatusUpdate) {

	for {
		s.Lock()
		logEvery := s.Config.HealthLogEvery
		s.Unlock()
		select {
		case <-ctx.Done():
			return
		case <-time.After(logEvery):
			updateHealth(updateChan) //force an update in case connections to relay and jump are down

			updateChan <- StatusUpdate{
				UpdateFunc: func(s *config.Status) {

					status, err := json.Marshal(s.Experiments)

					if err != nil {
						log.Errorf("HealthLogEvery marshal error %s", err.Error())
						return
					}
					log.WithField("status", string(status)).Infof("Current Health")
				},
			}

		}

	}
}

func processRelay(ctx context.Context, r *rc.Status, updateChan chan StatusUpdate) {

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
			updateFromRelay(reports, updateChan)
			updateHealth(updateChan)

		}
	}
}

func processJump(ctx context.Context, j *jc.Status, updateChan chan StatusUpdate) {

	for {
		select {
		case <-ctx.Done():
			return
		case reports, ok := <-j.Status:
			if !ok {
				log.Error("jump status channel not ok, stopping processing status")
				return
			}

			updateFromJump(reports, updateChan)
			updateHealth(updateChan)

		}

	}

}

func updateFromJump(reports []jc.Report, updateChan chan StatusUpdate) {

	updateChan <- StatusUpdate{
		UpdateFunc: func(s *config.Status) {

			now := s.Now()
			for _, r := range reports {

				// skip non-hosts
				if !slices.Contains(r.Scopes, "host") {
					continue
				}

				// initialise experiment's entry if not yet present
				if _, ok := s.Experiments[r.Topic]; !ok {
					s.Experiments[r.Topic] = NewReport(s.Now()) //pass status to get time
				}

				expt := s.Experiments[r.Topic]
				expt.JumpReport = r
				expt.LastCheckedJump = now
				expt.JumpOK = true //assume that a live connection is equivalent to a healthy one

				s.Experiments[r.Topic] = expt
			}
		},
	}
}

func updateFromRelay(reports []rc.Report, updateChan chan StatusUpdate) {
	updateChan <- StatusUpdate{
		UpdateFunc: func(s *config.Status) {
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
					s.Experiments[id] = NewReport(now)
				}

				stream := r.Topic

				expt := s.Experiments[id]

				expt.StreamReports[stream] = r

				expt.StreamOK[stream] = true

				if r.Stats.Tx.Last > s.Config.HealthLastActive {
					expt.StreamOK[stream] = false
				}

				if r.Stats.Tx.Never {
					expt.StreamOK[stream] = false
				}

				expt.LastCheckedStreams = now

				s.Experiments[id] = expt

			}
		},
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

// EmailBody creates the email body from the status
// access to status must be protected by a lock held by the calling function
func EmailBody(s *config.Status, alerts map[string]bool, systemAlerts []string) string {

	toHeader := strings.Join(s.Config.EmailTo, ",")
	ccHeader := strings.Join(s.Config.EmailCc, ",")

	// The msg parameter should be an RFC 822-style email with headers first,
	// a blank line, and then the message body. The lines of msg should be CRLF terminated.
	// The msg headers should usually include fields such as "From", "To", "Subject", and "Cc".

	count := strconv.Itoa(len(alerts))

	msg := "To: " + toHeader + "\r\n" +
		"Cc: " + ccHeader + "\r\n" +
		"Subject: " + s.Config.EmailSubject + " " + count + " new health events \r\n" +
		"\r\nSystem time: " + s.Now().String() + "\r\n"

	if len(systemAlerts) > 0 {
		msg += "\r\nSystem alerts:\r\n" + strings.Join(systemAlerts, "\r\n") + "\r\n"
	}

	amok := []string{}
	amissues := []string{}

	for k, v := range alerts {
		if v {
			amok = append(amok, k)
		} else {
			amissues = append(amissues, k)
		}
	}

	sort.Strings(amissues)
	sort.Strings(amok)

	msg += "There are " + count + " new health events (" +
		strconv.Itoa(len(amissues)) + " issues, " + strconv.Itoa(len(amok)) + " ok): \r\n"

	cav := " -- A   "
	cun := " ><   U "
	coka := " ok A   "  //🆗
	coku := " ok   U  " //🆗

	for _, k := range amissues {

		v := alerts[k]

		if v { //shouldn't get a positive value here for an issue
			code := coku
			if s.Experiments[k].Available {
				code = coka
			}
			msg += k + code + "\r\n"
		} else {

			code := cun
			if s.Experiments[k].Available {
				code = cav
			}
			he := s.Experiments[k].HealthEvents
			issues := "[unknown]"
			if len(he) > 0 {
				issues = "[" + strings.Join(he[len(he)-1].Issues, ", ") + "]"
			}
			msg += k + code + issues + "\r\n"
		}

	}

	for _, k := range amok {

		v := alerts[k]

		if v {
			code := coku
			if s.Experiments[k].Available {
				code = coka
			}
			msg += k + code + "\r\n"
		} else { //shouldn't get a negative value here for an ok

			code := cun
			if s.Experiments[k].Available {
				code = cav
			}
			he := s.Experiments[k].HealthEvents
			issues := "[unknown]"
			if len(he) > 0 {
				issues = "[" + strings.Join(he[len(he)-1].Issues, ", ") + "]"
			}
			msg += k + code + issues + "\r\n"
		}

	}

	msg += "\r\n\r\n All new and existing health issues:\r\n"

	hm := []string{}

	for k, v := range s.Experiments {
		if !v.Healthy {
			hm = append(hm, k)
		}
	}

	sort.Strings(hm)

	for _, k := range hm {

		v := s.Experiments[k]

		if !v.Healthy {
			code := cun
			if s.Experiments[k].Available {
				code = cav
			}
			he := v.HealthEvents
			issues := "[unknown]"
			if len(he) > 0 {
				issues = "[" + strings.Join(he[len(he)-1].Issues, ", ") + "]"
			}
			msg += k + code + issues + "\r\n"
		}
	}

	msg += "\r\n\r\n For the latest complete status information, please go to " + s.Config.EmailLink + "\r\n"

	return msg
}
