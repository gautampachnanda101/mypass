package main

import (
	"fmt"
	"os"

	"github.com/gautampachnanda101/vaultx/cmd/vaultx/commands"
)

// Injected by -ldflags at build time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	commands.SetBuildInfo(version, commit, date)
	if err := commands.Root().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
