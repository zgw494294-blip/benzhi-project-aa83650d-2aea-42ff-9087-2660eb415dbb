package webui

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"acousticverdictworkbench/internal/domain"
)

const maxBodyBytes = 1 << 20

type errorResponse struct {
	Error  string              `json:"error"`
	Field  string              `json:"field,omitempty"`
	Errors []domain.FieldError `json:"errors,omitempty"`
}

func decode(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return domain.Invalid("body", "请求 JSON 无效："+err.Error())
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return domain.Invalid("body", "请求体只能包含一个 JSON 对象")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeBatch(w http.ResponseWriter, status int, batch *domain.ReviewBatch, actorID, role string) {
	batch.Submissions = batch.VisibleSubmissions(actorID, role)
	batch.ReannotationTasks = batch.VisibleReannotationTasks(actorID, role)
	writeJSON(w, status, batch)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusUnprocessableEntity
	switch {
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, domain.ErrForbidden):
		status = http.StatusForbidden
	case errors.Is(err, domain.ErrTaskOwner):
		status = http.StatusForbidden
	case errors.Is(err, domain.ErrVersionConflict), errors.Is(err, domain.ErrStateConflict), errors.Is(err, domain.ErrAlreadyReleased), errors.Is(err, domain.ErrTaskRound), errors.Is(err, domain.ErrTaskClosed), errors.Is(err, domain.ErrIntegrity):
		status = http.StatusConflict
	}
	response := errorResponse{Error: err.Error()}
	var rule *domain.RuleError
	if errors.As(err, &rule) {
		response.Field = rule.Field
	}
	var integrity *domain.IntegrityError
	if errors.As(err, &integrity) {
		response.Field = integrity.Field
	}
	var validation *domain.ValidationErrors
	if errors.As(err, &validation) {
		response.Errors = validation.Errors
	}
	writeJSON(w, status, response)
}

func idempotencyKey(r *http.Request) (string, error) {
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		return "", domain.Invalid("Idempotency-Key", "写请求必须携带 Idempotency-Key")
	}
	if len(key) > 128 {
		return "", domain.Invalid("Idempotency-Key", "Idempotency-Key 不能超过 128 字符")
	}
	return key, nil
}
