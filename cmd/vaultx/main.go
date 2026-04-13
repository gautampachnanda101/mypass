package main

import (
	"fmt"
	"os"

	"github.com/gautampachnanda101/vaultx/cmd/vaultx/commands"
)

func main() {
	if err := commands.Root().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
