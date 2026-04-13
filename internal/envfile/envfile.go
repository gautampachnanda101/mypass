// Package envfile parses vaultx.env files — a strict superset of .env.
//
// Syntax:
//
//	KEY=plain_value            # literal string
//	KEY=vault:provider/path    # secret reference — resolved at runtime
//	KEY=${OTHER_KEY}           # interpolate from process environment
//	# comment
//	  # indented comment
//
// Plain values, vault references, and env interpolations may be mixed:
//
//	BASE_URL=https://api.example.com
//	DB_PASSWORD=vault:local/myapp/db
//	HOME_DIR=${HOME}
//
// File lookup order (first found wins):
//
//	./vaultx.env
//	./.vaultx.env
//	~/.vaultx/default.env
package envfile

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Entry is a single line parsed from a vaultx.env file.
type Entry struct {
	Key   string
	Value string // raw value as written in the file
	Kind  Kind
	Line  int // 1-based line number (for error messages)
}

// Kind describes how a value should be resolved.
type Kind int

const (
	// KindLiteral — plain string, used as-is.
	KindLiteral Kind = iota
	// KindRef — vault:provider/path reference, must be resolved by the resolver.
	KindRef
	// KindEnv — ${VAR} interpolation from the process environment.
	KindEnv
)

// File is a parsed vaultx.env file.
type File struct {
	Entries []Entry
	Path    string
}

// Refs returns only the entries that require vault resolution.
func (f *File) Refs() []Entry {
	var out []Entry
	for _, e := range f.Entries {
		if e.Kind == KindRef {
			out = append(out, e)
		}
	}
	return out
}

// Parse reads and parses a vaultx.env file from r.
func Parse(r io.Reader) (*File, error) {
	return ParseNamed(r, "")
}

// ParseNamed is like Parse but records the source path for error messages.
func ParseNamed(r io.Reader, path string) (*File, error) {
	f := &File{Path: path}
	scanner := bufio.NewScanner(r)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Strip inline comments (outside quotes).
		line = stripComment(line)
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			return nil, fmt.Errorf("%s:%d: missing '=' in %q", path, lineNum, line)
		}

		key := strings.TrimSpace(line[:eq])
		if key == "" {
			return nil, fmt.Errorf("%s:%d: empty key", path, lineNum)
		}
		if err := validateKey(key); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, lineNum, err)
		}

		rawVal := unquote(strings.TrimSpace(line[eq+1:]))
		kind := classifyValue(rawVal)

		f.Entries = append(f.Entries, Entry{
			Key:   key,
			Value: rawVal,
			Kind:  kind,
			Line:  lineNum,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}
	return f, nil
}

// ParseFile reads and parses the file at path.
func ParseFile(path string) (*File, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	return ParseNamed(fh, path)
}

// FindAndParse looks for a vaultx.env file in the standard locations and
// parses the first one found. Returns nil, nil if no file exists.
func FindAndParse() (*File, error) {
	candidates := []string{
		"vaultx.env",
		".vaultx.env",
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".vaultx", "default.env"))
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return ParseFile(p)
		}
	}
	return nil, nil
}

// classifyValue determines the Kind for a raw value string.
func classifyValue(v string) Kind {
	if strings.HasPrefix(v, "vault:") {
		return KindRef
	}
	if strings.HasPrefix(v, "${") && strings.HasSuffix(v, "}") {
		return KindEnv
	}
	return KindLiteral
}

// validateKey ensures the key is a valid env var name.
func validateKey(k string) error {
	for i, c := range k {
		switch {
		case c >= 'A' && c <= 'Z':
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9' && i > 0:
		case c == '_':
		default:
			return fmt.Errorf("invalid key %q: character %q not allowed", k, c)
		}
	}
	return nil
}

// unquote strips surrounding single or double quotes from a value.
func unquote(v string) string {
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') ||
			(v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	return v
}

// stripComment removes a trailing # comment, respecting quoted strings.
func stripComment(line string) string {
	inSingle, inDouble := false, false
	for i, c := range line {
		switch c {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble {
				return line[:i]
			}
		}
	}
	return line
}
