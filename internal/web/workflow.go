package web

import (
	"net/http"

	"phonemereleasedesk/internal/application"
	"phonemereleasedesk/internal/domain"
)

type assignmentRequest struct {
	ExpectedVersion uint64 `json:"expectedVersion"`
	SegmentID       string `json:"segmentId"`
	First           string `json:"firstAnnotatorId"`
	Second          string `json:"secondAnnotatorId"`
	ActorID         string `json:"actorId"`
	Role            string `json:"role"`
}

func (s *Server) AssignHandler(w http.ResponseWriter, r *http.Request) {
	var body assignmentRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	command := application.AssignmentCommand{Metadata: metadata(r, body.ExpectedVersion, body.ActorID, body.Role), BatchID: r.PathValue("id"), SegmentID: body.SegmentID, First: body.First, Second: body.Second}
	batch, err := s.service.Assign(r.Context(), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeBatch(w, http.StatusOK, batch)
}

type submissionRequest struct {
	ExpectedVersion uint64                   `json:"expectedVersion"`
	SegmentID       string                   `json:"segmentId"`
	AnnotatorID     string                   `json:"annotatorId"`
	Intervals       []domain.PhonemeInterval `json:"intervals"`
	Submit          bool                     `json:"submit"`
	ActorID         string                   `json:"actorId"`
	Role            string                   `json:"role"`
}

func (s *Server) SubmitAnnotationHandler(w http.ResponseWriter, r *http.Request) {
	var body submissionRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	command := application.SubmissionCommand{Metadata: metadata(r, body.ExpectedVersion, body.ActorID, body.Role), BatchID: r.PathValue("id"), SegmentID: body.SegmentID, AnnotatorID: body.AnnotatorID, Intervals: body.Intervals, Submit: body.Submit}
	batch, err := s.service.SaveSubmission(r.Context(), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeBatch(w, http.StatusOK, batch)
}

func (s *Server) OwnSubmissionHandler(w http.ResponseWriter, r *http.Request) {
	actor := r.URL.Query().Get("actorId")
	value, err := s.service.ViewOwnSubmission(r.Context(), r.PathValue("id"), r.PathValue("segment"), actor)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) RunCheckHandler(w http.ResponseWriter, r *http.Request) {
	var body batchRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	batch, err := s.service.RunCheck(r.Context(), application.BatchCommand{Metadata: metadata(r, body.ExpectedVersion, body.ActorID, body.Role), BatchID: r.PathValue("id")})
	if err != nil {
		writeError(w, err)
		return
	}
	writeBatch(w, http.StatusOK, batch)
}

type decisionRequest struct {
	ExpectedVersion uint64 `json:"expectedVersion"`
	SegmentID       string `json:"segmentId"`
	IntervalKey     string `json:"intervalKey"`
	ResolvedLabel   string `json:"resolvedLabel"`
	Reason          string `json:"reason"`
	AdjudicatorID   string `json:"adjudicatorId"`
	ActorID         string `json:"actorId"`
	Role            string `json:"role"`
}

func (s *Server) DecideHandler(w http.ResponseWriter, r *http.Request) {
	var body decisionRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	command := application.DecisionCommand{Metadata: metadata(r, body.ExpectedVersion, body.ActorID, body.Role), BatchID: r.PathValue("id"), SegmentID: body.SegmentID, IntervalKey: body.IntervalKey, ResolvedLabel: body.ResolvedLabel, Reason: body.Reason, AdjudicatorID: body.AdjudicatorID}
	batch, err := s.service.Decide(r.Context(), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeBatch(w, http.StatusOK, batch)
}
