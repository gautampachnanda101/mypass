package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gautampachnanda101/vaultx/internal/injector"
)

func cmdDocker() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docker",
		Short: "Run Docker commands with secrets injected (no env files)",
	}
	cmd.AddCommand(cmdDockerRun(), cmdDockerCompose())
	return cmd
}

func cmdDockerRun() *cobra.Command {
	return &cobra.Command{
		Use:                "run -- <docker-run-args>",
		Short:              "docker run with secrets injected as --env flags",
		Long:               `Resolves vaultx.env and prepends --env KEY=VALUE for each secret before calling docker run.
Secrets are passed as CLI arguments — they never touch the filesystem.`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && args[0] == "--" {
				args = args[1:]
			}
			if len(args) == 0 {
				return fmt.Errorf("usage: vaultx docker run -- <image> [args...]")
			}
			if err := requireUnlocked(); err != nil {
				return err
			}
			return injector.DockerRun(cmd.Context(), state.registry, globalFlags.envFile, args)
		},
	}
}

func cmdDockerCompose() *cobra.Command {
	return &cobra.Command{
		Use:                "compose -- <compose-args>",
		Short:              "docker compose with secrets in the child environment",
		Long:               `Resolves vaultx.env and runs docker compose with secrets exported into the
child process environment. Compose reads them automatically — no --env-file needed.`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && args[0] == "--" {
				args = args[1:]
			}
			if err := requireUnlocked(); err != nil {
				return err
			}
			return injector.DockerCompose(cmd.Context(), state.registry, globalFlags.envFile, args)
		},
	}
}
