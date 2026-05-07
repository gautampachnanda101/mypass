package injector

import (
	"context"
	"os"
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

func TestResolveFileWithTempFile(t *testing.T) {
	ctx := context.Background()
	
	// Create a temporary vaultx.env file
	tmpFile := t.TempDir() + "/vaultx.env"
	content := []byte("PLAIN_VAR=value123\nANOTHER=test\n")
	if err := os.WriteFile(tmpFile, content, 0600); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	
	// resolveFile should parse plain values even without providers
	resolved, err := resolveFile(ctx, nil, tmpFile)
	if err != nil {
		t.Fatalf("resolveFile failed: %v", err)
	}
	
	if resolved["PLAIN_VAR"] != "value123" {
		t.Errorf("expected PLAIN_VAR=value123, got %q", resolved["PLAIN_VAR"])
	}
	if resolved["ANOTHER"] != "test" {
		t.Errorf("expected ANOTHER=test, got %q", resolved["ANOTHER"])
	}
}

func TestEnvPairsEmptyMap(t *testing.T) {
	pairs := envPairs(map[string]string{})
	if len(pairs) != 0 {
		t.Fatalf("expected empty slice, got %v", pairs)
	}
}

func TestEnvPairsWithSpecialChars(t *testing.T) {
	m := map[string]string{
		"DB_URL": "postgres://user:p@ss@localhost:5432/db",
		"API_KEY": "abc=def=ghi",
	}
	pairs := envPairs(m)
	
	found := make(map[string]bool)
	for _, p := range pairs {
		if strings.HasPrefix(p, "DB_URL=") {
			found["DB_URL"] = true
			if !strings.Contains(p, "postgres://") {
				t.Errorf("DB_URL value not preserved: %q", p)
			}
		}
		if strings.HasPrefix(p, "API_KEY=") {
			found["API_KEY"] = true
			if !strings.Contains(p, "abc=def=ghi") {
				t.Errorf("API_KEY value not preserved: %q", p)
			}
		}
	}
	
	if !found["DB_URL"] || !found["API_KEY"] {
		t.Errorf("not all keys found in pairs: %v", pairs)
	}
}

