package errors

import (
	"fmt"

	"github.com/pkg/errors"
)

// ErrorType holds type of error.
type ErrorType uint

const (
	// NoType no typed error.
	NoType = ErrorType(iota)
	// BadRequest the server was unable to process the request sent by the client due to invalid syntax.
	BadRequest
	// NotFound the server could not find what was requested.
	NotFound
	// Conflict indicates that the request could not be processed because of conflict in the request,
	// such as an edit conflict in the case of multiple updates.
	Conflict
)

type pdnsError struct {
	errorType     ErrorType
	originalError error
	// context       errorContext
}

/*
type errorContext struct {
	Field   string
	Message string
}
*/

// Error returns the message of a PDNSError.
// Implements the error built-in interface.
func (e pdnsError) Error() string {
	return e.originalError.Error()
}

// New creates a new pdnsError
func (t ErrorType) New(msg string) error {
	return pdnsError{
		errorType:     t,
		originalError: errors.New(msg),
	}
}

// Newf creates a new pdnsError with formatted message
func (t ErrorType) Newf(msg string, args ...interface{}) error {
	e := fmt.Errorf(msg, args...)

	return pdnsError{
		errorType:     t,
		originalError: e,
	}
}

// Wrap creates a new wrapped error
func (t ErrorType) Wrap(err error, msg string) error {
	return t.Wrapf(err, msg)
}

// Wrapf creates a new wrapped error with formatted message
func (t ErrorType) Wrapf(err error, msg string, args ...interface{}) error {
	e := errors.Wrapf(err, msg, args...)

	return pdnsError{
		errorType:     t,
		originalError: e,
	}
}

// New creates a no type error
func New(msg string) error {
	return pdnsError{
		errorType:     NoType,
		originalError: errors.New(msg),
	}
}

// Newf creates a no type error with formatted message
func Newf(msg string, args ...interface{}) error {
	return pdnsError{
		errorType:     NoType,
		originalError: errors.New(fmt.Sprintf(msg, args...)),
	}
}

// Wrap wraps an error with a string
func Wrap(err error, msg string) error {
	return Wrapf(err, msg)
}

// Wrapf wraps an error with format string
func Wrapf(err error, msg string, args ...interface{}) error {
	wrappedError := errors.Wrapf(err, msg, args...)
	if pdnsErr, ok := err.(pdnsError); ok {
		return pdnsError{
			errorType:     pdnsErr.errorType,
			originalError: wrappedError,
			// context:       pdnsErr.context,
		}
	}

	return pdnsError{
		errorType:     NoType,
		originalError: wrappedError,
	}
}

// Cause gives the original error
func Cause(err error) error {
	return errors.Cause(err)
}

/*
// AddErrorContext adds a context to an error
func AddErrorContext(err error, field, message string) error {
	context := errorContext{
		Field:   field,
		Message: message,
	}

	if pdnsErr, ok := err.(pdnsError); ok {
		return pdnsError{
			errorType:     pdnsErr.errorType,
			originalError: pdnsErr.originalError,
			context:       context,
		}
	}

	return pdnsError{
		errorType:     NoType,
		originalError: err,
		context:       context,
	}
}

// GetErrorContext returns the error context
func GetErrorContext(err error) map[string]string {
	emptyContext := errorContext{}
	if pdnsErr, ok := err.(pdnsError); ok || pdnsErr.context != emptyContext {
		return map[string]string{
			"field":   pdnsErr.context.Field,
			"message": pdnsErr.context.Message,
		}
	}

	return nil
}
*/

// GetType returns the error type
func GetType(err error) ErrorType {
	if pdnsErr, ok := err.(pdnsError); ok {
		return pdnsErr.errorType
	}

	return NoType
}
