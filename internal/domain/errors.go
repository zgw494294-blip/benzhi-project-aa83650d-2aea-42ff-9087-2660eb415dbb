package domain

import "fmt"

var (
	ErrSealed          = fmt.Errorf("发布批次已封存，拒绝写入")
	ErrInvalidState    = fmt.Errorf("当前状态不允许此操作")
	ErrVersionConflict = fmt.Errorf("版本冲突")
	ErrNotFound        = fmt.Errorf("对象不存在")
	ErrForbidden       = fmt.Errorf("角色无权执行此操作")
)

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return e.Field + ": " + e.Message
}

func Invalid(field, message string) error {
	return ValidationError{Field: field, Message: message}
}
