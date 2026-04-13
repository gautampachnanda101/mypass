package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/gautampachnanda101/vaultx/internal/daemon"
)

func cmdServe() *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the local vaultx daemon (HTTP API for extensions and k3d)",
		Long: `Start a local HTTP daemon on 127.0.0.1:<port>.

The daemon exposes:
  GET  /health                      liveness check
  GET  /v1/secret?path=<path>       resolve a single secret
  POST /v1/resolve                  resolve a vaultx.env body
  GET  /v1/list?prefix=<p>          list secrets (values masked)
  GET  /externalsecrets/<key>       External Secrets Operator webhook

A session token is written to ~/.vaultx/daemon.token (mode 0600).
Pass it as the X-Vaultx-Token header or ?token= query param.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireUnlocked(); err != nil {
				return err
			}

			srv, err := daemon.New(state.registry, port)
			if err != nil {
				return fmt.Errorf("create daemon: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Token written to ~/.vaultx/daemon.token\n")

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			return srv.ListenAndServe(ctx)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 7474, "port to listen on")
	return cmd
}
