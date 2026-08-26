package web

import (
	"net/http"

	"phonemereleasedesk/internal/application"
)

type repairRequest struct {
	ExpectedVersion uint64 `json:"expectedVersion"`
	SegmentID       string `json:"segmentId"`
	Rule            string `json:"rule"`
	TargetKind      string `json:"targetKind"`
	AnnotatorID     string `json:"annotatorId,omitempty"`
	IntervalKey     string `json:"intervalKey,omitempty"`
	Reason          string `json:"reason"`
	ReviewerID      string `json:"reviewerId"`
	ActorID         string `json:"actorId"`
	Role            string `json:"role"`
}

func (s *Server) RepairHandler(w http.ResponseWriter, r *http.Request) {
	var body repairRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	command := application.RepairCommand{Metadata: metadata(r, body.ExpectedVersion, body.ActorID, body.Role), BatchID: r.PathValue("id"), SegmentID: body.SegmentID, Rule: body.Rule, TargetKind: body.TargetKind, AnnotatorID: body.AnnotatorID, IntervalKey: body.IntervalKey, Reason: body.Reason, ReviewerID: body.ReviewerID}
	batch, err := s.service.RequestRepair(r.Context(), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeBatch(w, http.StatusOK, batch)
}

type sealRequest struct {
	ExpectedVersion uint64 `json:"expectedVersion"`
	ReviewerID      string `json:"reviewerId"`
	ActorID         string `json:"actorId"`
	Role            string `json:"role"`
}

func (s *Server) SealHandler(w http.ResponseWriter, r *http.Request) {
	var body sealRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	command := application.SealCommand{Metadata: metadata(r, body.ExpectedVersion, body.ActorID, body.Role), BatchID: r.PathValue("id"), ReviewerID: body.ReviewerID}
	batch, err := s.service.Seal(r.Context(), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeBatch(w, http.StatusOK, batch)
}

func (s *Server) VerifyCredentialHandler(w http.ResponseWriter, r *http.Request) {
	result, err := s.service.VerifyCredential(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
