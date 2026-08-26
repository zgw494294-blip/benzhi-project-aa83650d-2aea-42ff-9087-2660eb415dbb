package domain

import "errors"

var (
	ErrNotFound        = errors.New("对象不存在")
	ErrValidation      = errors.New("输入校验失败")
	ErrVersionConflict = errors.New("版本冲突")
	ErrStateConflict   = errors.New("状态冲突")
	ErrForbidden       = errors.New("无权执行该操作")
	ErrAlreadyReleased = errors.New("批次已经封存")
	ErrTaskOwner       = errors.New("返标任务不属于当前标注员")
	ErrTaskRound       = errors.New("返标轮次不匹配")
	ErrTaskClosed      = errors.New("返标任务已经关闭")
	ErrIntegrity       = errors.New("发布清单完整性异常")
)

type RuleError struct {
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

type IntegrityError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *IntegrityError) Error() string { return e.Message }
func (e *IntegrityError) Unwrap() error { return ErrIntegrity }

func (e *RuleError) Error() string { return e.Message }

func Invalid(field, message string) error {
	return &RuleError{Field: field, Message: message}
}

func Integrity(field, message string) error {
	return &IntegrityError{Field: field, Message: message}
}

type FieldError struct {
	Row     int    `json:"row,omitempty"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationErrors struct {
	Errors []FieldError `json:"errors"`
}

func (e *ValidationErrors) Error() string { return ErrValidation.Error() }

func InvalidFields(errors []FieldError) error {
	return &ValidationErrors{Errors: errors}
}
