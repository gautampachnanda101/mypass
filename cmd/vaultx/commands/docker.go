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
		Long: "Resolve vault: references from vaultx.env and pass each secret as a\n" +
			"--env KEY=VALUE argument to Docker — nothing written to disk, nothing\n" +
			"visible in docker inspect env_file entries.",
		Example: `  vaultx docker run -- myapp:latest
  vaultx docker run -- -p 8080:8080 myapp:latest
  vaultx docker compose -- up -d
  vaultx docker compose -- up --build`,
	}
	cmd.AddCommand(cmdDockerRun(), cmdDockerCompose())
	return cmd
}

func cmdDockerRun() *cobra.Command {
	return &cobra.Command{
		Use:   "run -- <docker-run-args>",
		Short: "docker run with secrets injected as --env flags",
		Long: "Resolve vaultx.env and prepend --env KEY=VALUE for each secret before\n" +
			"calling docker run. Secrets are passed as CLI args — never on disk.",
		Example: `  vaultx docker run -- myapp:latest
  vaultx docker run -- -p 8080:8080 --name myapp myapp:latest`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
				return cmd.Help()
			}
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
		Use:   "compose -- <compose-args>",
		Short: "docker compose with secrets in the child environment",
		Long: "Resolve vaultx.env and run docker compose with secrets exported into\n" +
			"the child process environment. Compose reads them automatically —\n" +
			"no --env-file, no secrets on disk.",
		Example: `  vaultx docker compose -- up -d
  vaultx docker compose -- up --build
  vaultx docker compose -- down`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
				return cmd.Help()
			}
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
