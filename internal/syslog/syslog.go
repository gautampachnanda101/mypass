//go:build !windows
// +build !windows

package syslog

import (
	"fmt"
	"log/syslog"
	"sync"
)

// Writer sends audit events to a syslog server.
type Writer struct {
	mu     sync.Mutex
	writer *syslog.Writer
}

// New creates a syslog writer. Network can be "", "tcp", "udp".
// If network is empty, uses local syslog.
func New(network, address string) (*Writer, error) {
	var w *syslog.Writer
	var err error

	if network == "" {
		// Local syslog
		w, err = syslog.New(syslog.LOG_AUTH|syslog.LOG_INFO, "vaultx")
	} else {
		// Remote syslog
		w, err = syslog.Dial(network, address, syslog.LOG_AUTH|syslog.LOG_INFO, "vaultx")
	}

	if err != nil {
		return nil, fmt.Errorf("connect to syslog: %w", err)
	}

	return &Writer{writer: w}, nil
}

// Write sends a log message to syslog.
func (s *Writer) Write(msg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writer.Info(msg)
}

// WriteErr sends an error message to syslog.
func (s *Writer) WriteErr(msg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writer.Err(msg)
}

// Close closes the syslog connection.
func (s *Writer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writer.Close()
}
