package web

import (
	"net/http"

	"phonemereleasedesk/internal/application"
)

type createBatchRequest struct {
	ID                   string   `json:"id"`
	DialectSite          string   `json:"dialectSite"`
	PhoneticSystem       string   `json:"phoneticSystem"`
	AllowedLabels        []string `json:"allowedLabels"`
	MinimumSegmentMillis int64    `json:"minimumSegmentMillis"`
	RequireDual          bool     `json:"requireDual"`
	ActorID              string   `json:"actorId"`
	Role                 string   `json:"role"`
}

func (s *Server) CreateBatchHandler(w http.ResponseWriter, r *http.Request) {
	var body createBatchRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	command := application.CreateBatchCommand{Metadata: metadata(r, 0, body.ActorID, body.Role), ID: body.ID, DialectSite: body.DialectSite, PhoneticSystem: body.PhoneticSystem, AllowedLabels: body.AllowedLabels, MinimumSegmentMillis: body.MinimumSegmentMillis, RequireDual: body.RequireDual}
	batch, err := s.service.CreateBatch(r.Context(), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeBatch(w, http.StatusCreated, batch)
}

func (s *Server) ListBatchesHandler(w http.ResponseWriter, r *http.Request) {
	batches, err := s.service.ListBatches(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	for i := range batches {
		batches[i] = redactBatch(batches[i])
	}
	writeJSON(w, http.StatusOK, batches)
}

func (s *Server) GetBatchHandler(w http.ResponseWriter, r *http.Request) {
	batch, err := s.service.GetBatch(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeBatch(w, http.StatusOK, batch)
}

type segmentRequest struct {
	ExpectedVersion uint64 `json:"expectedVersion"`
	ID              string `json:"id"`
	SourceRef       string `json:"sourceRef"`
	StartMillis     int64  `json:"startMillis"`
	EndMillis       int64  `json:"endMillis"`
	SpeakerCode     string `json:"speakerCode"`
	PromptText      string `json:"promptText"`
	ActorID         string `json:"actorId"`
	Role            string `json:"role"`
}

func (s *Server) AddSegmentHandler(w http.ResponseWriter, r *http.Request) {
	var body segmentRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	command := application.SegmentCommand{Metadata: metadata(r, body.ExpectedVersion, body.ActorID, body.Role), BatchID: r.PathValue("id"), ID: body.ID, SourceRef: body.SourceRef, StartMillis: body.StartMillis, EndMillis: body.EndMillis, SpeakerCode: body.SpeakerCode, PromptText: body.PromptText}
	batch, err := s.service.AddSegment(r.Context(), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeBatch(w, http.StatusOK, batch)
}

type specificationRequest struct {
	ExpectedVersion      uint64   `json:"expectedVersion"`
	DialectSite          string   `json:"dialectSite"`
	PhoneticSystem       string   `json:"phoneticSystem"`
	AllowedLabels        []string `json:"allowedLabels"`
	MinimumSegmentMillis int64    `json:"minimumSegmentMillis"`
	RequireDual          bool     `json:"requireDual"`
	ActorID              string   `json:"actorId"`
	Role                 string   `json:"role"`
}

func (s *Server) UpdateSpecificationHandler(w http.ResponseWriter, r *http.Request) {
	var body specificationRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	command := application.SpecificationCommand{Metadata: metadata(r, body.ExpectedVersion, body.ActorID, body.Role), BatchID: r.PathValue("id"), DialectSite: body.DialectSite, PhoneticSystem: body.PhoneticSystem, AllowedLabels: body.AllowedLabels, MinimumSegmentMillis: body.MinimumSegmentMillis, RequireDual: body.RequireDual}
	batch, err := s.service.UpdateSpecification(r.Context(), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeBatch(w, http.StatusOK, batch)
}

func (s *Server) RemoveSegmentHandler(w http.ResponseWriter, r *http.Request) {
	var body batchRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	command := application.RemoveSegmentCommand{Metadata: metadata(r, body.ExpectedVersion, body.ActorID, body.Role), BatchID: r.PathValue("id"), SegmentID: r.PathValue("segment")}
	batch, err := s.service.RemoveSegment(r.Context(), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeBatch(w, http.StatusOK, batch)
}

type batchRequest struct {
	ExpectedVersion uint64 `json:"expectedVersion"`
	ActorID         string `json:"actorId"`
	Role            string `json:"role"`
}

func (s *Server) FreezeHandler(w http.ResponseWriter, r *http.Request) {
	var body batchRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	batch, err := s.service.Freeze(r.Context(), application.BatchCommand{Metadata: metadata(r, body.ExpectedVersion, body.ActorID, body.Role), BatchID: r.PathValue("id")})
	if err != nil {
		writeError(w, err)
		return
	}
	writeBatch(w, http.StatusOK, batch)
}
