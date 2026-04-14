package commands

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const publicGuideFile = "VAULTX_USER_GUIDE.md"

// embeddedPublicGuide is bundled into the binary so `vaultx docs` works even
// when the guide file is not present on disk (for example in restricted installs).
//go:embed VAULTX_USER_GUIDE.md
var embeddedPublicGuide string

func cmdDocs() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Pretty-print the public user guide shipped with the binary",
		Long: "Render the public vaultx user guide in your terminal with readable\n" +
			"headings and code blocks. By default, vaultx looks for " + publicGuideFile + "\n" +
			"next to the running binary and standard package doc paths. If not found,\n" +
			"it falls back to the embedded copy bundled in the binary.",
		Example: "  vaultx docs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, body, err := loadPublicGuide()
			if err != nil {
				return err
			}

			ux := uxFor(cmd)
			fmt.Fprintf(os.Stderr, "%s  Rendering docs from %s\n\n", icon(ux.Emoji, "info"), path)
			renderMarkdownPretty(cmd.OutOrStdout(), string(body), ux)
			return nil
		},
	}
	return cmd
}

func loadPublicGuide() (string, []byte, error) {
	var candidates []string
	if exe, err := os.Executable(); err == nil {
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, publicGuideFile),
			// Homebrew cellar installs docs under share/vaultx.
			filepath.Join(exeDir, "..", "share", "vaultx", publicGuideFile),
			filepath.Join(exeDir, "..", "..", "share", "vaultx", publicGuideFile),
		)
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, publicGuideFile))
	}

	for _, p := range candidates {
		if b, err := os.ReadFile(p); err == nil {
			return p, b, nil
		}
	}

	if strings.TrimSpace(embeddedPublicGuide) != "" {
		return "embedded:VAULTX_USER_GUIDE.md", []byte(embeddedPublicGuide), nil
	}

	return "", nil, fmt.Errorf("could not find %s and no embedded fallback is available", publicGuideFile)
}

func renderMarkdownPretty(w io.Writer, body string, ux terminalUX) {
	inCode := false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			inCode = !inCode
			if inCode {
				fmt.Fprintln(w, style(ux.Color, ansiDim, "--- code ---"))
			} else {
				fmt.Fprintln(w, style(ux.Color, ansiDim, "---"))
			}
			continue
		}

		if inCode {
			fmt.Fprintln(w, style(ux.Color, ansiDim, line))
			continue
		}

		switch {
		case strings.HasPrefix(trimmed, "# "):
			fmt.Fprintln(w, style(ux.Color, ansiBold+ansiCyan, strings.TrimPrefix(trimmed, "# ")))
		case strings.HasPrefix(trimmed, "## "):
			fmt.Fprintln(w)
			fmt.Fprintln(w, style(ux.Color, ansiBold+ansiBlue, strings.TrimPrefix(trimmed, "## ")))
		case strings.HasPrefix(trimmed, "### "):
			fmt.Fprintln(w, style(ux.Color, ansiBold+ansiMagenta, strings.TrimPrefix(trimmed, "### ")))
		case trimmed == "---":
			fmt.Fprintln(w, style(ux.Color, ansiDim, strings.Repeat("-", 60)))
		default:
			fmt.Fprintln(w, prettyInline(line, ux))
		}
	}
}

func prettyInline(s string, ux terminalUX) string {
	if strings.IndexByte(s, '`') < 0 {
		return s
	}
	var b strings.Builder
	inCode := false
	for _, r := range s {
		if r == '`' {
			inCode = !inCode
			if ux.Color {
				if inCode {
					b.WriteString(ansiGreen)
				} else {
					b.WriteString(ansiReset)
				}
			}
			continue
		}
		b.WriteRune(r)
	}
	if inCode && ux.Color {
		b.WriteString(ansiReset)
	}
	return b.String()
}