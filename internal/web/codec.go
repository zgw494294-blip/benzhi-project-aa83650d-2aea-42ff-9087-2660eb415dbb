package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"phonemereleasedesk/internal/application"
	"phonemereleasedesk/internal/domain"
)

type errorBody struct {
	Error    string               `json:"error"`
	Code     string               `json:"code"`
	Problems []domain.ItemProblem `json:"problems,omitempty"`
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "JSON 请求无效：" + err.Error(), Code: "invalid_json"})
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "请求体只能包含一个 JSON 对象", Code: "invalid_json"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func redactBatch(batch *domain.ReleaseBatch) *domain.ReleaseBatch {
	copyBatch := batch.Clone()
	for segmentID, list := range copyBatch.Submissions {
		for i := range list {
			list[i].Intervals = nil
		}
		copyBatch.Submissions[segmentID] = list
	}
	return copyBatch
}

func writeBatch(w http.ResponseWriter, status int, batch *domain.ReleaseBatch) {
	writeJSON(w, status, redactBatch(batch))
}

func writeError(w http.ResponseWriter, err error) {
	status, code := http.StatusUnprocessableEntity, "business_rule"
	switch {
	case errors.Is(err, domain.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, domain.ErrForbidden):
		status, code = http.StatusForbidden, "forbidden"
	case errors.Is(err, domain.ErrVersionConflict):
		status, code = http.StatusConflict, "version_conflict"
	case errors.Is(err, domain.ErrSealed):
		status, code = http.StatusConflict, "sealed"
	case errors.Is(err, domain.ErrInvalidState):
		status, code = http.StatusConflict, "invalid_state"
	}
	body := errorBody{Error: err.Error(), Code: code}
	var validation domain.BatchValidationError
	if errors.As(err, &validation) {
		body.Problems = validation.Problems
		code = "batch_validation"
		body.Code = code
	}
	writeJSON(w, status, body)
}

func metadata(r *http.Request, expected uint64, actor, role string) application.Metadata {
	if actor == "" {
		actor = strings.TrimSpace(r.Header.Get("X-Actor-ID"))
	}
	if role == "" {
		role = strings.TrimSpace(r.Header.Get("X-Role"))
	}
	return application.Metadata{ExpectedVersion: expected, IdempotencyKey: strings.TrimSpace(r.Header.Get("Idempotency-Key")), ActorID: actor, Role: role}
}
