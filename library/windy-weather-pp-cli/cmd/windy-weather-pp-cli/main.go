// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.
package main

import (
	"fmt"
	"os"

	"windy-weather-pp-cli/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(cli.ExitCode(err))
	}
}
