package web

import (
	"net/http"
	"strconv"
	"strings"

	"phonemereleasedesk/internal/application"
	"phonemereleasedesk/internal/domain"
)

type segmentBatchRequest struct {
	ExpectedVersion uint64                    `json:"expectedVersion"`
	ContentDigest   string                    `json:"contentDigest,omitempty"`
	Segments        []domain.RecordingSegment `json:"segments"`
	ActorID         string                    `json:"actorId"`
	Role            string                    `json:"role"`
}

func (s *Server) PreflightSegmentsHandler(w http.ResponseWriter, r *http.Request) {
	var body segmentBatchRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	result, err := s.service.PreflightSegments(r.Context(), application.SegmentBatchCommand{Metadata: metadata(r, body.ExpectedVersion, body.ActorID, body.Role), BatchID: r.PathValue("id"), Segments: body.Segments})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s *Server) AddSegmentsHandler(w http.ResponseWriter, r *http.Request) {
	var body segmentBatchRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	batch, err := s.service.AddSegments(r.Context(), application.SegmentBatchCommand{Metadata: metadata(r, body.ExpectedVersion, body.ActorID, body.Role), BatchID: r.PathValue("id"), ContentDigest: body.ContentDigest, Segments: body.Segments})
	if err != nil {
		writeError(w, err)
		return
	}
	writeBatch(w, http.StatusOK, batch)
}

type assignmentBatchRequest struct {
	ExpectedVersion uint64                      `json:"expectedVersion"`
	Plan            []domain.AssignmentPlanItem `json:"plan"`
	ActorID         string                      `json:"actorId"`
	Role            string                      `json:"role"`
}

func (s *Server) PreviewAssignmentsHandler(w http.ResponseWriter, r *http.Request) {
	var body assignmentBatchRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	result, err := s.service.PreviewAssignments(r.Context(), application.AssignmentBatchCommand{Metadata: metadata(r, body.ExpectedVersion, body.ActorID, body.Role), BatchID: r.PathValue("id"), Plan: body.Plan})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s *Server) AssignManyHandler(w http.ResponseWriter, r *http.Request) {
	var body assignmentBatchRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	batch, err := s.service.AssignMany(r.Context(), application.AssignmentBatchCommand{Metadata: metadata(r, body.ExpectedVersion, body.ActorID, body.Role), BatchID: r.PathValue("id"), Plan: body.Plan})
	if err != nil {
		writeError(w, err)
		return
	}
	writeBatch(w, http.StatusOK, batch)
}

func (s *Server) VerificationHistoryHandler(w http.ResponseWriter, r *http.Request) {
	runs, err := s.service.VerificationHistory(r.Context(), r.PathValue("id"), strings.TrimSpace(r.URL.Query().Get("rule")), strings.TrimSpace(r.URL.Query().Get("segmentId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs})
}
func (s *Server) CompareChecksHandler(w http.ResponseWriter, r *http.Request) {
	before, after := strings.TrimSpace(r.URL.Query().Get("beforeRunId")), strings.TrimSpace(r.URL.Query().Get("afterRunId"))
	if before == "" || after == "" {
		writeError(w, domain.Invalid("runId", "beforeRunId 和 afterRunId 均为必填"))
		return
	}
	result, err := s.service.CompareVerificationRuns(r.Context(), r.PathValue("id"), before, after, strings.TrimSpace(r.URL.Query().Get("rule")), strings.TrimSpace(r.URL.Query().Get("segmentId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) DecisionQueueHandler(w http.ResponseWriter, r *http.Request) {
	pending := r.URL.Query().Get("pending") != "false"
	items, err := s.service.DecisionQueue(r.Context(), r.PathValue("id"), strings.TrimSpace(r.URL.Query().Get("segmentId")), strings.TrimSpace(r.URL.Query().Get("candidateLabel")), pending)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
}

type decisionBatchRequest struct {
	ExpectedVersion uint64                `json:"expectedVersion"`
	Decisions       []domain.BulkDecision `json:"decisions"`
	ActorID         string                `json:"actorId"`
	Role            string                `json:"role"`
}

func (s *Server) DecideManyHandler(w http.ResponseWriter, r *http.Request) {
	var body decisionBatchRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	batch, err := s.service.DecideMany(r.Context(), application.DecisionBatchCommand{Metadata: metadata(r, body.ExpectedVersion, body.ActorID, body.Role), BatchID: r.PathValue("id"), Decisions: body.Decisions})
	if err != nil {
		writeError(w, err)
		return
	}
	writeBatch(w, http.StatusOK, batch)
}

func (s *Server) RepairTasksHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	items, err := s.service.RepairTasks(r.Context(), r.PathValue("id"), application.RepairTaskQuery{SegmentID: strings.TrimSpace(q.Get("segmentId")), Rule: strings.TrimSpace(q.Get("rule")), TargetKind: strings.TrimSpace(q.Get("targetKind")), Status: strings.TrimSpace(q.Get("status")), ActorID: strings.TrimSpace(q.Get("actorId")), Role: strings.TrimSpace(q.Get("role"))})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": items, "count": len(items)})
}

func (s *Server) CredentialDetailHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, ok := queryInt(q.Get("page"), 1)
	if !ok {
		writeError(w, domain.Invalid("page", "页码必须为正整数"))
		return
	}
	pageSize, ok := queryInt(q.Get("pageSize"), 50)
	if !ok {
		writeError(w, domain.Invalid("pageSize", "每页数量必须为正整数"))
		return
	}
	result, err := s.service.CredentialDetail(r.Context(), application.CredentialLookup{CredentialID: strings.TrimSpace(q.Get("credentialId")), BatchID: strings.TrimSpace(q.Get("batchId")), SegmentID: strings.TrimSpace(q.Get("segmentId")), Page: page, PageSize: pageSize})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s *Server) VerifyCredentialDetailHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	result, err := s.service.VerifyCredentialDetail(r.Context(), strings.TrimSpace(q.Get("credentialId")), strings.TrimSpace(q.Get("batchId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func queryInt(value string, fallback int) (int, bool) {
	if strings.TrimSpace(value) == "" {
		return fallback, true
	}
	number, err := strconv.Atoi(value)
	return number, err == nil && number > 0
}
