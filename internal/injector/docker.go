// Package injector provides secret injection into Docker containers and
// docker-compose stacks without writing secrets to disk.
//
// How it works:
//
//  1. vaultx resolves all vault: references from vaultx.env.
//  2. For docker run: secrets are passed as --env KEY=VALUE args to the
//     docker CLI subprocess. They never touch the filesystem and are not
//     visible in `docker inspect` (unlike --env-file).
//  3. For docker-compose: vaultx exec's the compose command with the
//     resolved secrets in the child process environment — compose inherits
//     them and injects them into containers at startup.
//
// Usage:
//
//	vaultx docker run [--env vaultx.env] -- <docker-run-args>
//	vaultx docker compose [--env vaultx.env] -- up -d
package injector

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/gautampachnanda101/vaultx/internal/envfile"
	"github.com/gautampachnanda101/vaultx/internal/resolver"
)

// DockerRun executes `docker run` with secrets injected as --env flags.
// args should be the arguments that follow `docker run` (image, cmd, etc.).
// Secrets from the resolved env file are prepended as --env KEY=VALUE flags.
// This replaces the current process via syscall.Exec — no fork.
func DockerRun(ctx context.Context, reg *resolver.Registry, envFilePath string, args []string) error {
	resolved, err := resolveFile(ctx, reg, envFilePath)
	if err != nil {
		return err
	}

	dockerArgs := buildDockerRunArgs(resolved, args)
	return execDocker(ctx, dockerArgs)
}

// DockerCompose executes a docker-compose command with the resolved env
// exported into the child process. Compose reads from the environment
// automatically, so no explicit --env-file is needed.
// args should be everything after `docker compose` (e.g. ["up", "-d"]).
func DockerCompose(ctx context.Context, reg *resolver.Registry, envFilePath string, args []string) error {
	resolved, err := resolveFile(ctx, reg, envFilePath)
	if err != nil {
		return err
	}

	// Build child env: current env + resolved secrets.
	childEnv := append(os.Environ(), envPairs(resolved)...)

	composeArgs := append([]string{"compose"}, args...)
	binary, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("docker not found in PATH")
	}

	// Replace current process — secrets only in memory.
	return syscall.Exec(binary, append([]string{"docker"}, composeArgs...), childEnv)
}

// buildDockerRunArgs prepends --env KEY=VALUE flags for each resolved secret
// and returns the complete argument slice for `docker run ...`.
func buildDockerRunArgs(resolved map[string]string, userArgs []string) []string {
	args := make([]string, 0, 2*len(resolved)+len(userArgs))
	for k, v := range resolved {
		args = append(args, "--env", k+"="+v)
	}
	return append(args, userArgs...)
}

// execDocker resolves the docker binary and replaces the current process.
func execDocker(_ context.Context, args []string) error {
	binary, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("docker not found in PATH")
	}
	return syscall.Exec(binary, append([]string{"docker", "run"}, args...), os.Environ())
}

// resolveFile parses and resolves a vaultx.env file.
// If envFilePath is empty it falls back to auto-detection.
func resolveFile(ctx context.Context, reg *resolver.Registry, envFilePath string) (map[string]string, error) {
	var f *envfile.File
	var err error

	if envFilePath != "" {
		f, err = envfile.ParseFile(envFilePath)
	} else {
		f, err = envfile.FindAndParse()
	}
	if err != nil {
		return nil, fmt.Errorf("parse env file: %w", err)
	}
	if f == nil {
		return map[string]string{}, nil
	}
	return reg.Resolve(ctx, f)
}

// envPairs converts a map to "KEY=VALUE" strings for os/exec Env fields.
func envPairs(m map[string]string) []string {
	pairs := make([]string, 0, len(m))
	for k, v := range m {
		pairs = append(pairs, k+"="+v)
	}
	return pairs
}
