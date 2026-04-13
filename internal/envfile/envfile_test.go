package envfile

import (
	"strings"
	"testing"
)

func parse(t *testing.T, src string) *File {
	t.Helper()
	f, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return f
}

func TestLiteral(t *testing.T) {
	f := parse(t, "PORT=3000\n")
	if len(f.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(f.Entries))
	}
	e := f.Entries[0]
	if e.Key != "PORT" || e.Value != "3000" || e.Kind != KindLiteral {
		t.Fatalf("unexpected entry: %+v", e)
	}
}

func TestVaultRef(t *testing.T) {
	f := parse(t, "DB_PASSWORD=vault:local/myapp/db\n")
	e := f.Entries[0]
	if e.Kind != KindRef || e.Value != "vault:local/myapp/db" {
		t.Fatalf("unexpected entry: %+v", e)
	}
}

func TestEnvInterpolation(t *testing.T) {
	f := parse(t, "HOME_DIR=${HOME}\n")
	e := f.Entries[0]
	if e.Kind != KindEnv || e.Value != "${HOME}" {
		t.Fatalf("unexpected entry: %+v", e)
	}
}

func TestComments(t *testing.T) {
	src := `
# full line comment
PORT=3000        # inline comment
DB=vault:local/db
`
	f := parse(t, src)
	if len(f.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %+v", len(f.Entries), f.Entries)
	}
}

func TestQuotedValues(t *testing.T) {
	src := `
A="hello world"
B='single quoted'
C=unquoted
`
	f := parse(t, src)
	if f.Entries[0].Value != "hello world" {
		t.Fatalf("double-quoted: got %q", f.Entries[0].Value)
	}
	if f.Entries[1].Value != "single quoted" {
		t.Fatalf("single-quoted: got %q", f.Entries[1].Value)
	}
	if f.Entries[2].Value != "unquoted" {
		t.Fatalf("unquoted: got %q", f.Entries[2].Value)
	}
}

func TestHashInQuotedValueNotStripped(t *testing.T) {
	f := parse(t, `PASS="p#ssword"`)
	if f.Entries[0].Value != "p#ssword" {
		t.Fatalf("hash inside quoted value should not be treated as comment: got %q", f.Entries[0].Value)
	}
}

func TestMixedFile(t *testing.T) {
	src := `
# vaultx.env — safe to commit
DB_PASSWORD=vault:local/myapp/db_password
API_KEY=vault:1password/Work/stripe
JWT_SECRET=vault:aws/myapp/jwt
PORT=3000
HOME_DIR=${HOME}
`
	f := parse(t, src)
	if len(f.Entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(f.Entries))
	}

	refs := f.Refs()
	if len(refs) != 3 {
		t.Fatalf("expected 3 vault refs, got %d", len(refs))
	}
}

func TestMissingEquals(t *testing.T) {
	_, err := Parse(strings.NewReader("NOEQUALS\n"))
	if err == nil {
		t.Fatal("expected parse error for missing '='")
	}
}

func TestEmptyKey(t *testing.T) {
	_, err := Parse(strings.NewReader("=value\n"))
	if err == nil {
		t.Fatal("expected parse error for empty key")
	}
}

func TestInvalidKeyCharacter(t *testing.T) {
	_, err := Parse(strings.NewReader("MY-KEY=value\n"))
	if err == nil {
		t.Fatal("expected parse error for hyphen in key")
	}
}

func TestEmptyLinesAndWhitespace(t *testing.T) {
	src := "\n\n  \nKEY=val\n\n"
	f := parse(t, src)
	if len(f.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(f.Entries))
	}
}

func TestLineNumbers(t *testing.T) {
	src := "A=1\nB=2\nC=3\n"
	f := parse(t, src)
	for i, e := range f.Entries {
		if e.Line != i+1 {
			t.Fatalf("entry %d: expected line %d, got %d", i, i+1, e.Line)
		}
	}
}
