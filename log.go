// Package log is a logging package that prints errors with optional tags.
//
// Log is similar to the Go standard library log package, with the following major
// changes:
//
//	- Log has a flag to log as JSON.
//	- Log unwraps errors and prints key/value pairs (tags) if they implement the
//	  fur.Tagger interface.
//	- Log only has Printf-variants, not Println and Print, but always writes an
//	  ending newline.
//
// Example usage:
//
//	firstErr := ...
//	otherError := fur.Errorf("connect to remote: %w", firstErr).Tag("address", address)
//	...
//	err := fur.Errorf("open resource: %w", otherErr).Tag("id", 123)
//	...
//	log.Printf("processing request: %w", err)
//
// You will have to imagine the errors being returned by different functions.
// You'll see a complete error message logged, along with key/value pairs "address"
// and "id".
package log

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"golang.org/x/xerrors"

	"github.com/mjl-/log/fur"
)

var (
	std = New(os.Stderr, "", 0)
)

// SetFlags sets new flags for the default logger.
// Default is 0.
func SetFlags(flags int) {
	std.SetFlags(flags)
}

// SetPrefix sets a new prefix for the default logger.
// Default is the empty string.
func SetPrefix(prefix string) {
	std.SetPrefix(prefix)
}

// SetOutput sets a new output for the default logger.
// Default is os.Stderr.
func SetOutput(out io.Writer) {
	std.SetOutput(out)
}

// Printf logs a message to the default logger.
func Printf(format string, args ...interface{}) {
	std.write(format, args, 2)
}

// Fatalf logs a message to the default logger and quits the program with exit status 1.
func Fatalf(format string, args ...interface{}) {
	std.write(format, args, 2)
	os.Exit(1)
}

// Panicf logs a message to the default logger and calls panic.
func Panicf(format string, args ...interface{}) {
	s := std.write(format, args, 2)
	panic(s)
}

// Logger provides functions for logging.
type Logger struct {
	out    io.Writer
	prefix string
	flags  int
}

// FlagTimestamp and other flags influence the fields written to out for a log message.
const (
	FlagTimestamp = 1 << iota // Print timestamp in local time zone, formatted with time.RFC3339Nano.
	FlagUTC                   // If printing timestamp, print in UTC.
	FlagFile                  // Filename with line number.
	FlagPath                  // Full path with line number.

	// Log a message on a single line in JSON format, with fields "message",
	// "timestamp", "file", "level" ("info" or "error", depending on whether the
	// message contains a wrapped error) and all (unwrapped) tags.
	FlagJSON
)

// New returns a new logger.
func New(out io.Writer, prefix string, flags int) *Logger {
	return &Logger{
		out:    out,
		prefix: prefix,
		flags:  flags,
	}
}

// SetFlags modifies log printing flags on the logger.
func (l *Logger) SetFlags(flags int) {
	l.flags = flags
}

// SetPrefix sets a new prefix.
func (l *Logger) SetPrefix(prefix string) {
	l.prefix = prefix
}

// SetOutput sets a new writer where logs will be written to.
// Log does one write at a time, with text ending in a newline.
func (l *Logger) SetOutput(out io.Writer) {
	l.out = out
}

// Printf formats its parameters and prints them.
func (l *Logger) Printf(format string, args ...interface{}) {
	l.write(format, args, 2)
}

// Fatalf prints a log message and exits the program.
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.write(format, args, 2)
	os.Exit(1)
}

// Panicf prints a log message and calls panic.
func (l *Logger) Panicf(format string, args ...interface{}) {
	s := l.write(format, args, 2)
	panic(s)
}

func (l *Logger) write(format string, args []interface{}, calldepth int) string {
	if l.flags&FlagJSON != 0 {
		return l.writeJSON(format, args, calldepth+1)
	}

	b := &strings.Builder{}
	if l.flags&FlagTimestamp != 0 {
		now := time.Now()
		if l.flags&FlagUTC != 0 {
			now = now.UTC()
		}
		b.WriteString(now.Format(time.RFC3339Nano) + " ")
	}

	b.WriteString(l.prefix)

	if l.flags&(FlagFile|FlagPath) != 0 {
		_, file, line, ok := runtime.Caller(calldepth)
		if ok {
			if l.flags&FlagPath == 0 {
				_, file = path.Split(file)
			}
			fmt.Fprintf(b, "%s:%d: ", file, line)
		}
	}

	err := xerrors.Errorf(format, args...)
	b.WriteString(err.Error())
	err = xerrors.Unwrap(err)

	prefix := " ("
	for ; err != nil; err = xerrors.Unwrap(err) {
		e, ok := err.(fur.Tagger)
		if !ok {
			continue
		}
		for k, v := range e.Tags() {
			fmt.Fprintf(b, "%s%s=%v", prefix, k, v)
			prefix = " "
		}
	}
	if prefix != " (" {
		b.WriteString(")")
	}
	s := b.String()
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	l.out.Write([]byte(s))
	return s
}

func (l *Logger) writeJSON(format string, args []interface{}, calldepth int) string {
	o := map[string]interface{}{}

	err := xerrors.Errorf(format, args...)
	msg := err.Error()
	o["message"] = msg

	if l.flags&FlagTimestamp != 0 {
		now := time.Now()
		if l.flags&FlagUTC != 0 {
			now = now.UTC()
		}
		o["timestamp"] = now.Format(time.RFC3339Nano)
	}

	if l.flags&(FlagFile|FlagPath) != 0 {
		_, file, line, ok := runtime.Caller(calldepth)
		if ok {
			if l.flags&FlagPath == 0 {
				_, file = path.Split(file)
			}
			o["file"] = fmt.Sprintf("%s:%d: ", file, line)
		}
	}

	err = xerrors.Unwrap(err)
	if err == nil {
		o["level"] = "info"
	} else {
		o["level"] = "error"
	}
	for ; err != nil; err = xerrors.Unwrap(err) {
		e, ok := err.(fur.Tagger)
		if !ok {
			continue
		}
		for k, v := range e.Tags() {
			o[k] = v
		}
	}

	buf, err := json.Marshal(o)
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal json for message %q: %s\n", msg, err)
		return msg
	}
	s := string(buf)
	l.out.Write([]byte(s + "\n"))
	return s
}
