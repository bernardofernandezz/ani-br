package errorx

import (
	"errors"
	"fmt"
)

type ErrorCode string

const (
	CodeUnknown           ErrorCode = "unknown"
	CodeInvalidArgument   ErrorCode = "invalid_argument"
	CodeDependencyMissing ErrorCode = "dependency_missing"
	CodeNetwork           ErrorCode = "network"
	CodeProvider          ErrorCode = "provider"
	CodeNotFound          ErrorCode = "not_found"
)

// AppError é um erro tipado com mensagem amigável (PT-BR) + erro técnico wrappado.
type AppError struct {
	Code    ErrorCode
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" && e.Err != nil {
		return e.Err.Error()
	}
	if e.Err == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Err)
}

func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func IsCode(err error, code ErrorCode) bool {
	var app *AppError
	if !errors.As(err, &app) {
		return false
	}
	return app.Code == code
}

