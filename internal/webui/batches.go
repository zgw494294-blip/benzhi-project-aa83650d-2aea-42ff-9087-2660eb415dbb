package webui

import (
	"net/http"

	"acousticverdictworkbench/internal/application"
	"acousticverdictworkbench/internal/domain"
)

func (s *Server) ListBatchesHandler(w http.ResponseWriter, r *http.Request) {
	batches, err := s.service.ListBatches(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	for _, batch := range batches {
		batch.Submissions = nil
		batch.ReannotationTasks = nil
	}
	writeJSON(w, http.StatusOK, map[string]any{"batches": batches})
}

func (s *Server) GetBatchHandler(w http.ResponseWriter, r *http.Request) {
	viewer, role := actor(r)
	view, err := s.service.GetBatch(r.Context(), r.PathValue("batchID"), viewer, role)
	if err != nil {
		writeError(w, err)
		return
	}
	// 对外详情使用按身份过滤的 submissions，避免草稿从 batch 字段旁路泄漏。
	view.Batch.Submissions = nil
	view.Batch.ReannotationTasks = nil
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) BulkRegisterClipsHandler(w http.ResponseWriter, r *http.Request) {
	key, err := idempotencyKey(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var cmd application.BulkRegisterClipsCommand
	if err := decode(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	cmd.BatchID, cmd.IdempotencyKey = r.PathValue("batchID"), key
	if len(cmd.Clips) == 0 || len(cmd.Clips) > 200 {
		writeError(w, domain.Invalid("clips", "批量登记条数必须在 1 到 200 之间"))
		return
	}
	result, err := s.service.BulkRegisterClips(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	result.Batch.Submissions = result.Batch.VisibleSubmissions(cmd.ActorID, cmd.Role)
	result.Batch.ReannotationTasks = result.Batch.VisibleReannotationTasks(cmd.ActorID, cmd.Role)
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) CreateBatchHandler(w http.ResponseWriter, r *http.Request) {
	key, err := idempotencyKey(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var cmd application.CreateBatchCommand
	if err := decode(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	cmd.IdempotencyKey = key
	batch, err := s.service.CreateBatch(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeBatch(w, http.StatusCreated, batch, cmd.ActorID, cmd.Role)
}

func (s *Server) ConfigureScopeHandler(w http.ResponseWriter, r *http.Request) {
	key, err := idempotencyKey(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var cmd application.ConfigureScopeCommand
	if err := decode(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	cmd.BatchID, cmd.IdempotencyKey = r.PathValue("batchID"), key
	batch, err := s.service.ConfigureScope(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeBatch(w, http.StatusOK, batch, cmd.ActorID, cmd.Role)
}

func (s *Server) AddClipHandler(w http.ResponseWriter, r *http.Request) {
	key, err := idempotencyKey(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var cmd application.AddClipCommand
	if err := decode(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	cmd.BatchID, cmd.IdempotencyKey = r.PathValue("batchID"), key
	batch, err := s.service.AddClip(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeBatch(w, http.StatusCreated, batch, cmd.ActorID, cmd.Role)
}

func (s *Server) RemoveClipHandler(w http.ResponseWriter, r *http.Request) {
	key, err := idempotencyKey(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var cmd application.BatchCommand
	if err := decode(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	cmd.BatchID, cmd.IdempotencyKey = r.PathValue("batchID"), key
	batch, err := s.service.RemoveClip(r.Context(), cmd, r.PathValue("clipID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeBatch(w, http.StatusOK, batch, cmd.ActorID, cmd.Role)
}

func (s *Server) FreezeBatchHandler(w http.ResponseWriter, r *http.Request) {
	key, err := idempotencyKey(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var cmd application.BatchCommand
	if err := decode(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	cmd.BatchID, cmd.IdempotencyKey = r.PathValue("batchID"), key
	batch, err := s.service.Freeze(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeBatch(w, http.StatusOK, batch, cmd.ActorID, cmd.Role)
}
