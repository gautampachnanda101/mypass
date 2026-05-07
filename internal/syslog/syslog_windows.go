//go:build windows
// +build windows

package syslog

import "fmt"

// Writer is a no-op syslog writer for Windows (syslog not supported).
type Writer struct{}

// New returns a no-op writer on Windows since syslog is Unix-only.
func New(network, address string) (*Writer, error) {
	return nil, fmt.Errorf("syslog not supported on Windows")
}

// Write is a no-op on Windows.
func (s *Writer) Write(msg string) error {
	return fmt.Errorf("syslog not supported on Windows")
}

// WriteErr is a no-op on Windows.
func (s *Writer) WriteErr(msg string) error {
	return fmt.Errorf("syslog not supported on Windows")
}
