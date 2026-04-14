package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const publicGuideFile = "VAULTX_USER_GUIDE.md"

func cmdDocs() *cobra.Command {
	var guidePath string

	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Pretty-print the public user guide shipped with the binary",
		Long: "Render the public vaultx user guide in your terminal with readable\n" +
			"headings and code blocks. By default, vaultx looks for " + publicGuideFile + "\n" +
			"next to the running binary, then in the current directory.\n\n" +
			"Use --file to print a specific guide file.",
		Example: "  vaultx docs\n" +
			"  vaultx docs --file ./VAULTX_USER_GUIDE.md",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, body, err := loadPublicGuide(guidePath)
			if err != nil {
				return err
			}

			ux := uxFor(cmd)
			fmt.Fprintf(os.Stderr, "%s  Rendering docs from %s\n\n", icon(ux.Emoji, "info"), path)
			renderMarkdownPretty(cmd.OutOrStdout(), string(body), ux)
			return nil
		},
	}

	cmd.Flags().StringVarP(&guidePath, "file", "f", "", "path to a markdown guide file")
	return cmd
}

func loadPublicGuide(override string) (string, []byte, error) {
	if strings.TrimSpace(override) != "" {
		b, err := os.ReadFile(override)
		if err != nil {
			return "", nil, fmt.Errorf("read guide %s: %w", override, err)
		}
		return override, b, nil
	}

	var candidates []string
	if exe, err := os.Executable(); err == nil {
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), publicGuideFile))
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(cwd, publicGuideFile),
			filepath.Join(cwd, "docs", "user-guide.md"),
		)
	}

	for _, p := range candidates {
		if b, err := os.ReadFile(p); err == nil {
			return p, b, nil
		}
	}

	return "", nil, fmt.Errorf("could not find %s near the binary or in the current directory (try: vaultx docs --file /path/to/%s)", publicGuideFile, publicGuideFile)
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