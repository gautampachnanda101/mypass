package commands

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/gautampachnanda101/vaultx/internal/keychain"
	"github.com/spf13/cobra"
	"net/http"
	"io"
)

func cmdAudit() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "View and export audit logs from the vaultx daemon",
		Long: `Query audit logs from a running vaultx daemon.

Audit logs track all security-relevant operations: secret access, authentication
attempts, unlock failures, and policy violations.

The daemon must be running for this command to work.`,
		Example: `  # View recent audit events (default: 100)
  vaultx audit

  # View last 500 events
  vaultx audit --limit 500

  # Export to JSON
  vaultx audit --format json --output audit.json

  # Export to CSV
  vaultx audit --format csv --output audit.csv`,
	}

	var (
		limit  int
		format string
		output string
		port   int
	)

	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum number of audit events to retrieve")
	cmd.Flags().StringVar(&format, "format", "table", "Output format: table, json, csv")
	cmd.Flags().StringVar(&output, "output", "", "Output file (default: stdout)")
	cmd.Flags().IntVar(&port, "port", 7474, "Daemon port")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		// Load daemon token from keychain
		token, ok := keychain.LoadDaemonToken()
		if !ok {
			return fmt.Errorf("daemon token not found - is the daemon running?")
		}

		// Fetch audit logs from daemon
		url := fmt.Sprintf("http://127.0.0.1:%d/v1/audit?limit=%d", port, limit)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("X-Vaultx-Token", token)

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("daemon request failed (is daemon running on port %d?): %w", port, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("daemon returned %d: %s", resp.StatusCode, string(body))
		}

		var result struct {
			Events []AuditEvent `json:"events"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}

		// Determine output destination
		writer := os.Stdout
		if output != "" {
			f, err := os.Create(output)
			if err != nil {
				return fmt.Errorf("create output file: %w", err)
			}
			defer f.Close()
			writer = f
		}

		// Format and write output
		switch format {
		case "json":
			enc := json.NewEncoder(writer)
			enc.SetIndent("", "  ")
			if err := enc.Encode(result.Events); err != nil {
				return fmt.Errorf("encode json: %w", err)
			}

		case "csv":
			w := csv.NewWriter(writer)
			defer w.Flush()

			// Write CSV header
			if err := w.Write([]string{"Timestamp", "Action", "Path", "RemoteAddr", "Success", "Error"}); err != nil {
				return err
			}

			// Write rows
			for _, e := range result.Events {
				row := []string{
					e.Timestamp.Format(time.RFC3339),
					e.Action,
					e.Path,
					e.RemoteAddr,
					strconv.FormatBool(e.Success),
					e.Error,
				}
				if err := w.Write(row); err != nil {
					return err
				}
			}

		case "table":
			fmt.Fprintf(writer, "%-23s %-20s %-30s %-18s %s\n", "TIMESTAMP", "ACTION", "PATH", "REMOTE", "STATUS")
			fmt.Fprintf(writer, "%s\n", "--------------------------------------------------------------------------------------------")
			for _, e := range result.Events {
				status := "✓"
				if !e.Success {
					status = "✗ " + e.Error
				}
				fmt.Fprintf(writer, "%-23s %-20s %-30s %-18s %s\n",
					e.Timestamp.Format("2006-01-02 15:04:05"),
					e.Action,
					truncate(e.Path, 30),
					e.RemoteAddr,
					truncate(status, 40))
			}

		default:
			return fmt.Errorf("unknown format: %s (use table, json, or csv)", format)
		}

		if output != "" {
			fmt.Fprintf(os.Stderr, "Exported %d audit events to %s\n", len(result.Events), output)
		}

		return nil
	}

	return cmd
}

// AuditEvent mirrors the structure in internal/daemon/daemon.go
type AuditEvent struct {
	Timestamp  time.Time `json:"timestamp"`
	Action     string    `json:"action"`
	Path       string    `json:"path,omitempty"`
	RemoteAddr string    `json:"remote_addr"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
