package webui

import (
	"net/http"
	"strconv"
	"strings"

	"acousticverdictworkbench/internal/application"
	"acousticverdictworkbench/internal/domain"
)

func (s *Server) ManifestDetailsHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if len(query.Get("clipId")) > 128 || len(query.Get("speciesCode")) > 32 {
		writeError(w, domain.Invalid("filter", "筛选字段长度超限"))
		return
	}
	page, err := positiveQueryInt(query.Get("page"), 1)
	if err != nil {
		writeError(w, domain.Invalid("page", err.Error()))
		return
	}
	pageSize, err := positiveQueryInt(query.Get("pageSize"), 20)
	if err != nil {
		writeError(w, domain.Invalid("pageSize", err.Error()))
		return
	}
	start, err := optionalInt64(query.Get("startMs"))
	if err != nil {
		writeError(w, domain.Invalid("startMs", "startMs 必须是整数"))
		return
	}
	end, err := optionalInt64(query.Get("endMs"))
	if err != nil {
		writeError(w, domain.Invalid("endMs", "endMs 必须是整数"))
		return
	}
	actorID, role := actor(r)
	details, err := s.service.ManifestDetails(r.Context(), r.PathValue("batchID"), application.Metadata{ActorID: actorID, Role: role}, application.ManifestEventQuery{ClipID: query.Get("clipId"), SpeciesCode: query.Get("speciesCode"), StartMs: start, EndMs: end, Page: page, PageSize: pageSize})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, details)
}

func (s *Server) VerifyManifestHandler(w http.ResponseWriter, r *http.Request) {
	actorID, role := actor(r)
	result, err := s.service.VerifyManifest(r.Context(), r.PathValue("batchID"), application.Metadata{ActorID: actorID, Role: role})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func positiveQueryInt(raw string, fallback int) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, domain.Invalid("query", "分页参数必须为正整数")
	}
	return value, nil
}

func optionalInt64(raw string) (*int64, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func (s *Server) ResolveDisputeHandler(w http.ResponseWriter, r *http.Request) {
	key, err := idempotencyKey(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var cmd application.ResolveCommand
	if err := decode(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	cmd.BatchID, cmd.DisputeID, cmd.IdempotencyKey = r.PathValue("batchID"), r.PathValue("disputeID"), key
	batch, err := s.service.Resolve(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeBatch(w, http.StatusOK, batch, cmd.ActorID, cmd.Role)
}

func (s *Server) QualityCheckHandler(w http.ResponseWriter, r *http.Request) {
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
	batch, err := s.service.CheckQuality(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeBatch(w, http.StatusOK, batch, cmd.ActorID, cmd.Role)
}

func (s *Server) ReleaseBatchHandler(w http.ResponseWriter, r *http.Request) {
	key, err := idempotencyKey(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var cmd application.ReleaseCommand
	if err := decode(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	cmd.BatchID, cmd.IdempotencyKey = r.PathValue("batchID"), key
	batch, err := s.service.Release(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeBatch(w, http.StatusOK, batch, cmd.ActorID, cmd.Role)
}
