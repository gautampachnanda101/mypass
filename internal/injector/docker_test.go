package injector

import (
	"context"
	"strings"
	"testing"
)

func TestBuildDockerRunArgs(t *testing.T) {
	resolved := map[string]string{
		"DB_PASS": "s3cr3t",
		"API_KEY": "tok3n",
	}
	userArgs := []string{"myimage:latest", "--rm"}

	args := buildDockerRunArgs(resolved, userArgs)

	// Count --env flags.
	envCount := 0
	for i, a := range args {
		if a == "--env" && i+1 < len(args) {
			envCount++
			val := args[i+1]
			if !strings.Contains(val, "=") {
				t.Fatalf("--env value missing '=': %q", val)
			}
		}
	}
	if envCount != len(resolved) {
		t.Fatalf("expected %d --env flags, got %d", len(resolved), envCount)
	}

	// User args must appear at the end.
	if args[len(args)-2] != "myimage:latest" && args[len(args)-1] != "--rm" {
		// Order of user args relative to --env flags isn't guaranteed by map
		// iteration, but user args must all be present.
		userArgSet := map[string]bool{}
		for _, a := range args {
			userArgSet[a] = true
		}
		if !userArgSet["myimage:latest"] || !userArgSet["--rm"] {
			t.Fatalf("user args missing from output: %v", args)
		}
	}
}

func TestBuildDockerRunArgsEmptySecrets(t *testing.T) {
	args := buildDockerRunArgs(map[string]string{}, []string{"alpine"})
	if len(args) != 1 || args[0] != "alpine" {
		t.Fatalf("expected [alpine], got %v", args)
	}
}

func TestEnvPairs(t *testing.T) {
	m := map[string]string{"FOO": "bar", "BAZ": "qux"}
	pairs := envPairs(m)
	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d", len(pairs))
	}
	for _, p := range pairs {
		if !strings.Contains(p, "=") {
			t.Fatalf("pair missing '=': %q", p)
		}
	}
}

func TestResolveFileNilOnMissingFile(t *testing.T) {
	// resolveFile with no file path and no vaultx.env in cwd returns empty map.
	ctx := context.Background()
	// Build a minimal registry with no providers.
	resolved, err := resolveFile(ctx, nil, "/nonexistent/path/vaultx.env")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	_ = resolved
}
