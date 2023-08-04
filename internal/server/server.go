package server

import (
	"context"

	jc "github.com/practable/jump/pkg/status"
	rc "github.com/practable/relay/pkg/status"
	"github.com/practable/status/internal/config"
	"github.com/practable/status/internal/serve"
	"github.com/practable/status/internal/status"
)

// Run starts all goroutines/services need to run a status process
func Run(ctx context.Context, c config.Config) {

	j := jc.New()
	r := rc.New()
	s := config.New()
	s.Config = c

	run(ctx, j, r, s)

}

// run starts all services needed to run a status process
// it accepts status clients and status object as parameters
// to aid mocking for testing
func run(ctx context.Context, j *jc.Status, r *rc.Status, s *config.Status) {

	go status.Run(ctx, j, r, s)
	go serve.API(ctx, s)
	<-ctx.Done()

}
