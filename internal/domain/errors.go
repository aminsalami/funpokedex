package domain

import (
	"fmt"
	"net/http"
)

type AppError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
	Cause      error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

func ErrNotFound(name string) *AppError {
	return &AppError{
		Code:       "POKEMON_NOT_FOUND",
		Message:    fmt.Sprintf("pokemon %q not found", name),
		HTTPStatus: http.StatusNotFound,
	}
}

func ErrUpstream(cause error) *AppError {
	return &AppError{
		Code:       "UPSTREAM_ERROR",
		Message:    "upstream service unavailable",
		HTTPStatus: http.StatusBadGateway,
		Cause:      cause,
	}
}

func ErrBadRequest(msg string) *AppError {
	return &AppError{
		Code:       "BAD_REQUEST",
		Message:    msg,
		HTTPStatus: http.StatusBadRequest,
	}
}
