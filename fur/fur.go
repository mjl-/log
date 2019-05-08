// Package fur provides an Error type that can be tagged with key/value pairs.
// Log unwraps errors to print the error message along with tags.
package fur

import (
	"golang.org/x/xerrors"
)

// Tags is a map with key/value pairs as fields to an error.
type Tags map[string]interface{}

// Tagger is an interface for errors with tags.
type Tagger interface {
	Tags() Tags
}

// Error is an error with Tags.
type Error struct {
	Err  error
	tags Tags
}

// Error returns the error message for the underlying error, tags are not included.
func (e Error) Error() string {
	return e.Err.Error()
}

// Unwrap unwraps the underlying error.
func (e Error) Unwrap() error {
	return xerrors.Unwrap(e.Err)
}

// Tags returns the tags for this error.
func (e Error) Tags() Tags {
	return e.tags
}

// Tag sets key to value, modifying the error.
func (e Error) Tag(key string, value interface{}) Error {
	e.tags[key] = value
	return e
}

// Errorf formats a new Error with empty tags.
func Errorf(format string, args ...interface{}) Error {
	err := xerrors.Errorf(format, args...)
	return Error{err, Tags{}}
}

// New returns a new Error with empty tags.
func New(err error) Error {
	return Error{err, Tags{}}
}
