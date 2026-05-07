//go:build windows || plan9

// Package syslog provides audit log forwarding to syslog servers.
package syslog

import (
	"errors"
	"sync"
)

// Writer is a stub on platforms where log/syslog is unavailable.
type Writer struct {
	mu sync.Mutex
}

var errNotSupported = errors.New("syslog not supported on this platform")

// New always returns an error on Windows and plan9.
func New(_, _ string) (*Writer, error) {
	return nil, errNotSupported
}

func (s *Writer) Write(_ string) error    { return errNotSupported }
func (s *Writer) WriteErr(_ string) error { return errNotSupported }
func (s *Writer) Close() error            { return nil }
