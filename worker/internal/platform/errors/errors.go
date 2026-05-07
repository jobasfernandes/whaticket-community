package errors

import "fmt"

type AppError struct {
	Code    string
	Status  int
	wrapped error
}

func (e *AppError) Error() string {
	if e.wrapped != nil {
		return fmt.Sprintf("%s: %v", e.Code, e.wrapped)
	}
	return e.Code
}

func (e *AppError) Unwrap() error {
	return e.wrapped
}

func New(code string, status int) *AppError {
	return &AppError{Code: code, Status: status}
}

func Wrap(err error, code string, status int) *AppError {
	return &AppError{Code: code, Status: status, wrapped: err}
}
