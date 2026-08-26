package domain

import "errors"

var (
	ErrNotFound          = errors.New("记录不存在")
	ErrValidation        = errors.New("字段校验失败")
	ErrStateConflict     = errors.New("当前状态不允许此操作")
	ErrVersionConflict   = errors.New("版本冲突")
	ErrAlreadyExists     = errors.New("记录已存在")
	ErrImmutable         = errors.New("验收案已封存，不可修改")
	ErrEvidenceConflict  = errors.New("当前读数已被其他修订替代")
	ErrCredentialMissing = errors.New("当前验收案尚无已批准凭据")
)

type FieldError struct{ Field, Message string }

func (e *FieldError) Error() string { return e.Field + ": " + e.Message }
func (e *FieldError) Unwrap() error { return ErrValidation }

type ConflictError struct{ Expected, Actual int64 }

func (e *ConflictError) Error() string { return "预期版本与当前版本不一致" }
func (e *ConflictError) Unwrap() error { return ErrVersionConflict }
