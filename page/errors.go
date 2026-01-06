package page

import (
	"errors"
	"io"
	"time"
)

const MaxStackDepth = 50

// Error manages panics during request handling.
type Error struct {
	// the underlying error
	Err error
	// the time the error occurred
	Time time.Time
	// unwound Stack info
	Stack []StackFrame
}

func (e *Error) Unwrap() error {
	return e.Err
}

// StackFrame holds the file, line and function name in a call chain
type StackFrame struct {
	File string
	Line int
	Func string
}

// NoErr represents no error. A request starts with this.
type NoErr struct {
}

func (e *NoErr) Error() string {
	return ""
}
func (e *NoErr) HttpStatus() int {
	return 200
}

// ErrNoTemplate indicates a template does not exist.
// The control will move on to other ways of rendering.
type ErrNoTemplate struct{}

func (ErrNoTemplate) Error() string { return "Form or control does not have a template" }
func IsNoTemplateError(err error) bool {
	var e ErrNoTemplate
	return errors.As(err, &e)
}
func NoTemplateError() error {
	return ErrNoTemplate{}
}

// ErrRecordNotFound is a rare situation that might come up as a race condition error between viewing a
// record, and actually editing it. If in the time between clicking on a record to see detail, and viewing the detail,
// the record was deleted by another user, we would return this error.
// In a REST environment, this is 404 error
type ErrRecordNotFound struct{}

func (ErrRecordNotFound) Error() string   { return "Record does not exist" }
func (ErrRecordNotFound) HttpStatus() int { return 404 }
func IsRecordNotFoundError(err error) bool {
	var e ErrRecordNotFound
	return errors.As(err, &e)
}
func RecordNotFoundError() error {
	return ErrRecordNotFound{}
}

type HttpStatuser interface {
	HttpStatus() int
}

func HttpStatus(err error) int {
	if i, ok := err.(HttpStatuser); ok {
		return i.HttpStatus()
	}
	return 500
}

// WriteString is a utility function that will write a string and panic if an error occurs
func WriteString(w io.Writer, s string) {
	if _, err := io.WriteString(w, s); err != nil {
		panic(err)
	}
}
