package errors

import (
	"errors"
	"fmt"
)

type NotFoundError struct{ Resource, ID string }
type ValidationError struct{ Message string }
type InternalError struct {
	Op  string
	Err error
}

// InvalidInputError carries a machine-readable code alongside a human-readable message,
// for endpoints whose API contract exposes distinct 400 error codes (e.g. INVALID_DATE_RANGE).
type InvalidInputError struct {
	Code    string
	Message string
}

// UpstreamError wraps a failure from a downstream service call (e.g. parking-service).
type UpstreamError struct {
	Op  string
	Err error
}

// ConflictError signals that a request could not be completed because it collides with
// existing state (e.g. an overlapping booking for the same time range).
type ConflictError struct{ Message string }

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s with id %q not found", e.Resource, e.ID)
}
func (e *ValidationError) Error() string   { return e.Message }
func (e *InternalError) Error() string     { return fmt.Sprintf("%s: %v", e.Op, e.Err) }
func (e *InternalError) Unwrap() error     { return e.Err }
func (e *InvalidInputError) Error() string { return e.Message }
func (e *UpstreamError) Error() string     { return fmt.Sprintf("%s: %v", e.Op, e.Err) }
func (e *UpstreamError) Unwrap() error     { return e.Err }
func (e *ConflictError) Error() string     { return e.Message }

func NewNotFound(resource, id string) error       { return &NotFoundError{Resource: resource, ID: id} }
func NewValidation(msg string) error              { return &ValidationError{Message: msg} }
func NewInternalError(op string, err error) error { return &InternalError{Op: op, Err: err} }
func NewInvalidInput(code, message string) error {
	return &InvalidInputError{Code: code, Message: message}
}
func NewUpstream(op string, err error) error { return &UpstreamError{Op: op, Err: err} }
func NewConflict(message string) error       { return &ConflictError{Message: message} }

func IsNotFound(err error) bool   { _, ok := err.(*NotFoundError); return ok }
func IsValidation(err error) bool { _, ok := err.(*ValidationError); return ok }

func IsConflict(err error) bool {
	var target *ConflictError
	return errors.As(err, &target)
}

func IsInvalidInput(err error) bool {
	var target *InvalidInputError
	return errors.As(err, &target)
}

func IsUpstream(err error) bool {
	var target *UpstreamError
	return errors.As(err, &target)
}

// InvalidInputCode extracts the machine-readable code from an InvalidInputError,
// or "" if err is not one.
func InvalidInputCode(err error) string {
	var target *InvalidInputError
	if errors.As(err, &target) {
		return target.Code
	}
	return ""
}
