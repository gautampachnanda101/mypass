package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/gautampachnanda101/vaultx/internal/daemon"
)

const defaultMaxMemMB = 64

func cmdServe() *cobra.Command {
	var port int
	var maxMemMB int

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the local vaultx daemon (HTTP API for extensions and k3d)",
		Long: "Start a local HTTP daemon on 127.0.0.1:<port>.\n\n" +
			"Endpoints:\n" +
			"  GET  /health                      liveness check (no auth)\n" +
			"  GET  /v1/secret?path=<path>       resolve a single secret\n" +
			"  POST /v1/resolve                  resolve a vaultx.env body\n" +
			"  GET  /v1/list?prefix=<p>          list secrets (values masked)\n" +
			"  GET  /externalsecrets/<key>       External Secrets Operator webhook\n\n" +
			"A session token is written to ~/.vaultx/daemon.token (mode 0600).\n" +
			"Pass it as the X-Vaultx-Token header or ?token= query param.\n\n" +
			"Memory: the daemon uses GOGC=off + a soft memory limit (default 64 MiB).\n" +
			"GC only runs when the heap approaches the limit, keeping idle CPU near zero.\n" +
			"Override with --max-memory.",
		Example: "  vaultx serve\n" +
			"  vaultx serve --port 8080\n" +
			"  vaultx serve --max-memory 32\n\n" +
			"  # Resolve a secret via the daemon\n" +
			"  TOKEN=$(cat ~/.vaultx/daemon.token)\n" +
			"  curl -H \"X-Vaultx-Token: $TOKEN\" http://localhost:7474/v1/secret?path=myapp/db",
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Pin GC to trigger only at the memory limit, not on a timer.
			// This keeps idle CPU at zero and avoids unnecessary page activity.
			debug.SetGCPercent(-1) // equivalent to GOGC=off
			limitBytes := int64(maxMemMB) * 1024 * 1024
			debug.SetMemoryLimit(limitBytes)

			if err := requireUnlocked(); err != nil {
				return err
			}

			srv, err := daemon.New(state.registry, port)
			if err != nil {
				return fmt.Errorf("create daemon: %w", err)
			}

			ux := uxFor(cmd)
			fmt.Fprintf(os.Stderr, "%s  Listening on 127.0.0.1:%d  (memory limit: %d MiB)\n",
				icon(ux.Emoji, "ok"), port, maxMemMB)
			fmt.Fprintf(os.Stderr, "%s  Token: ~/.vaultx/daemon.token\n", icon(ux.Emoji, "info"))

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			return srv.ListenAndServe(ctx)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 7474, "port to listen on")
	cmd.Flags().IntVar(&maxMemMB, "max-memory", defaultMaxMemMB, "soft memory limit for the daemon in MiB")
	return cmd
}
