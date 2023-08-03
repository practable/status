package serve

import (
	"context"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/runtime/middleware"
	"github.com/icza/gog"
	"github.com/practable/status/internal/config"
	"github.com/practable/status/internal/serve/models"
	"github.com/practable/status/internal/serve/restapi"
	"github.com/practable/status/internal/serve/restapi/operations"
	log "github.com/sirupsen/logrus"
)

func API(ctx context.Context, status *config.Status) {
	defer func() {
		log.Trace("serve.API stopped")
	}()

	swaggerSpec, err := loads.Embedded(restapi.SwaggerJSON, restapi.FlatSwaggerJSON)
	if err != nil {
		log.Fatalln(err)
	}

	api := operations.NewServeAPI(swaggerSpec)
	server := restapi.NewServer(api)

	server.Port = status.Config.Port

	// set the Handlers
	api.StatusExperimentsHandler = operations.StatusExperimentsHandlerFunc(statusExperimentsHandler(status))
	api.HealthEventsHandler = operations.HealthEventsHandlerFunc(healthEventsHandler(status))

	go func() {
		defer func() {
			log.Trace("status.serve.API cancel checker goro stopped")
		}()
		log.Trace("status.serve.API awaiting context cancellation")
		<-ctx.Done()
		log.Trace("status.serve.API context cancelled")
		if err := server.Shutdown(); err != nil {
			log.Fatalln(err)
		}

	}()

	server.ConfigureAPI()

	if err := server.Serve(); err != nil {
		log.Fatalln(err)
	}

	log.Trace("status.serve.API stopped without error")

}

func statusExperimentsHandler(s *config.Status) func(operations.StatusExperimentsParams) middleware.Responder {

	return func(params operations.StatusExperimentsParams) middleware.Responder {
		// convert current status into models version

		s.Lock()
		defer s.Unlock()

		sm := []*models.ExperimentReport{}
		for k, v := range s.Experiments {

			jr := models.JumpReport{
				Connected: gog.Ptr(v.JumpReport.Connected.String()),
				ExpiresAt: gog.Ptr(v.JumpReport.ExpiresAt.String()),
				Scopes:    v.JumpReport.Scopes,
				Stats: gog.Ptr(models.RxTx{
					Rx: &models.Statistics{
						Fps: int64(v.JumpReport.Stats.Rx.FPS), Last: v.JumpReport.Stats.Rx.Last.String(),
						Never: v.JumpReport.Stats.Rx.Never,
						Size:  int64(v.JumpReport.Stats.Rx.Size),
					},
					Tx: &models.Statistics{
						Fps:   int64(v.JumpReport.Stats.Tx.FPS),
						Last:  v.JumpReport.Stats.Tx.Last.String(),
						Never: v.JumpReport.Stats.Tx.Never,
						Size:  int64(v.JumpReport.Stats.Tx.Size),
					},
				}),
				Topic:     gog.Ptr(v.JumpReport.Topic),
				UserAgent: gog.Ptr(v.JumpReport.UserAgent),
			}

			srs := make(map[string]models.StreamReport)

			for _, srv := range v.StreamReports {

				sr := models.StreamReport{
					Connected: gog.Ptr(srv.Connected.String()),
					ExpiresAt: gog.Ptr(srv.ExpiresAt.String()),
					Scopes:    srv.Scopes,
					Stats: gog.Ptr(models.RxTx{
						Rx: &models.Statistics{
							Fps: int64(srv.Stats.Rx.FPS), Last: srv.Stats.Rx.Last.String(),
							Never: srv.Stats.Rx.Never,
							Size:  int64(srv.Stats.Rx.Size),
						},
						Tx: &models.Statistics{
							Fps:   int64(srv.Stats.Tx.FPS),
							Last:  srv.Stats.Tx.Last.String(),
							Never: srv.Stats.Tx.Never,
							Size:  int64(srv.Stats.Tx.Size),
						},
					}),
					Topic:     gog.Ptr(srv.Topic),
					UserAgent: gog.Ptr(srv.UserAgent),
				}

				srs[srv.Topic] = sr

			}

			r := models.ExperimentReport{
				Available:           gog.Ptr(v.Available),
				FirstChecked:        gog.Ptr(v.FirstChecked.String()),
				HealthEvents:        int64(len(v.HealthEvents)), // GET /experiments/health/{topic_name} to to get health events
				Healthy:             gog.Ptr(v.Healthy),
				JumpOk:              gog.Ptr(v.JumpOK),
				JumpReport:          &jr,
				LastCheckedJump:     gog.Ptr(v.LastCheckedJump.String()),
				LastCheckedStreams:  gog.Ptr(v.LastCheckedStreams.String()),
				LastFoundInManifest: gog.Ptr(v.LastFoundInManifest.String()),
				ResourceName:        gog.Ptr(v.ResourceName),
				StreamOk:            v.StreamOK,
				StreamReports:       srs,
				StreamRequired:      v.StreamRequired,
				TopicName:           gog.Ptr(k),
			}
			sm = append(sm, &r)

		}

		// return
		return operations.NewStatusExperimentsOK().WithPayload(sm)
	}

}

func healthEventsHandler(s *config.Status) func(operations.HealthEventsParams) middleware.Responder {

	return func(params operations.HealthEventsParams) middleware.Responder {

		s.Lock()
		defer s.Unlock()

		// check topic name
		if _, ok := s.Experiments[params.Name]; !ok {
			return operations.NewHealthEventsNotFound()
		}

		hes := []*models.HealthEvent{}

		for _, v := range s.Experiments[params.Name].HealthEvents {

			he := models.HealthEvent{
				Healthy:  v.Healthy,
				Issues:   v.Issues,
				JumpOk:   v.JumpOK,
				StreamOk: v.StreamOK,
				When:     v.When.String(),
			}
			hes = append(hes, &he)

		}

		return operations.NewHealthEventsOK().WithPayload(hes)

	}

}
