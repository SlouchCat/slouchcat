package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The guided wizard behind the login (concept 22 part A, x-visibility owner): its state, one step
// at a time, and the finish. The admin service does the work.

func (e serviceProblem) VisitGetSetupProgressResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitUpdateSetupProgressResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitFinishSetupResponse(w http.ResponseWriter) error { return e.visit(w) }

func (s *server) GetSetupProgress(ctx context.Context, _ gen.GetSetupProgressRequestObject) (gen.GetSetupProgressResponseObject, error) {
	p, err := s.opts.Admin.SetupProgress(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetSetupProgress200JSONResponse(toSetupProgress(p)), nil
}

func (s *server) UpdateSetupProgress(ctx context.Context, req gen.UpdateSetupProgressRequestObject) (gen.UpdateSetupProgressResponseObject, error) {
	b := deref(req.Body)
	upd := model.SetupProgressUpdate{TrickplayDefault: b.TrickplayDefault}
	if b.Step != nil {
		step := string(*b.Step)
		upd.Step = &step
	}
	if b.State != nil {
		state := string(*b.State)
		upd.State = &state
	}
	p, err := s.opts.Admin.UpdateSetupProgress(ctx, upd)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.UpdateSetupProgress200JSONResponse(toSetupProgress(p)), nil
}

func (s *server) FinishSetup(ctx context.Context, _ gen.FinishSetupRequestObject) (gen.FinishSetupResponseObject, error) {
	session, ok := sessionFrom(ctx)
	if !ok {
		// The owner gate only lets requests with a session through.
		return nil, errors.New("finishSetup reached without a session")
	}
	done, err := s.opts.Admin.FinishSetup(ctx, session.Account.ID)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.FinishSetup200JSONResponse(gen.SetupFinished{Restart: gen.SetupFinishedRestart(done.Restart), RestartAt: optTime(done.RestartAt)}), nil
}

func toSetupProgress(p model.SetupProgress) gen.SetupProgress {
	out := gen.SetupProgress{Steps: make([]gen.SetupStepEntry, 0, len(p.Steps)), TrickplayDefault: p.TrickplayDefault,
		Finished: p.Finished, FinishedAt: optTime(p.FinishedAt)}
	for _, st := range p.Steps {
		out.Steps = append(out.Steps, gen.SetupStepEntry{Step: gen.SetupStep(st.Step), State: gen.SetupStepState(st.State)})
	}
	return out
}
