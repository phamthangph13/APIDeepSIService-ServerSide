package domain

import (
	"errors"
	"fmt"
	"net/http"
)

// Error is the application-level error type. Services return it so the HTTP
// layer can map failures to stable error codes without knowing their details.
type Error struct {
	Status  int
	Code    string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Err }

func NewError(status int, code, message string, err error) *Error {
	return &Error{Status: status, Code: code, Message: message, Err: err}
}

func Invalid(code, message string) *Error {
	return NewError(http.StatusBadRequest, code, message, nil)
}

func Unauthorized(code, message string) *Error {
	return NewError(http.StatusUnauthorized, code, message, nil)
}

func Forbidden(code, message string) *Error {
	return NewError(http.StatusForbidden, code, message, nil)
}

func NotFound(code, message string) *Error {
	return NewError(http.StatusNotFound, code, message, nil)
}

func Conflict(code, message string) *Error {
	return NewError(http.StatusConflict, code, message, nil)
}

func TooManyRequests(code, message string) *Error {
	return NewError(http.StatusTooManyRequests, code, message, nil)
}

func Internal(err error) *Error {
	return NewError(http.StatusInternalServerError, "internal_error", "Đã có lỗi xảy ra. Vui lòng thử lại.", err)
}

var (
	ErrNotFound      = errors.New("not found")
	ErrDuplicate     = errors.New("duplicate")
	ErrInvalid       = errors.New("invalid")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrForbidden     = errors.New("forbidden")
	ErrExpired       = errors.New("expired")
	ErrQuotaExceeded = errors.New("quota exceeded")
)

// AsError converts any error into an *Error, preserving application errors and
// hiding internal details from clients.
func AsError(err error) *Error {
	if err == nil {
		return nil
	}
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	switch {
	case errors.Is(err, ErrNotFound):
		return NotFound("not_found", "Không tìm thấy dữ liệu.")
	case errors.Is(err, ErrDuplicate):
		return Conflict("already_exists", "Dữ liệu này đã tồn tại.")
	case errors.Is(err, ErrExpired):
		return NewError(http.StatusGone, "expired", "Liên kết đã hết hạn.", err)
	default:
		return Internal(err)
	}
}
