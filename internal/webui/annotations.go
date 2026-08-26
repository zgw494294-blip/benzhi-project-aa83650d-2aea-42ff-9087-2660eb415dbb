package webui

import (
	"net/http"

	"acousticverdictworkbench/internal/application"
)

func (s *Server) SaveDraftHandler(w http.ResponseWriter, r *http.Request) {
	key, err := idempotencyKey(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var cmd application.DraftCommand
	if err := decode(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	cmd.BatchID, cmd.ClipID, cmd.IdempotencyKey = r.PathValue("batchID"), r.PathValue("clipID"), key
	batch, err := s.service.SaveDraft(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeBatch(w, http.StatusOK, batch, cmd.ActorID, cmd.Role)
}

func (s *Server) SubmitAnnotationHandler(w http.ResponseWriter, r *http.Request) {
	key, err := idempotencyKey(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var cmd application.SubmitCommand
	if err := decode(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	cmd.BatchID, cmd.ClipID, cmd.IdempotencyKey = r.PathValue("batchID"), r.PathValue("clipID"), key
	batch, err := s.service.Submit(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeBatch(w, http.StatusOK, batch, cmd.ActorID, cmd.Role)
}
