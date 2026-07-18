// Package cli wires the local-first Trade Republic command surface.
package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

const Version = "0.1.0"

type flags struct {
	JSON, Agent, Quiet, NoInput bool
	Config, Database            string
	Timeout                     time.Duration
}

type codedError struct {
	code int
	err  error
}

func (e codedError) Error() string { return e.err.Error() }
func (e codedError) Unwrap() error { return e.err }

func Execute() error { return RootCmd().Execute() }

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var coded codedError
	if errors.As(err, &coded) {
		return coded.code
	}
	return 1
}

func RootCmd() *cobra.Command {
	f := &flags{Timeout: 5 * time.Minute}
	root := &cobra.Command{
		Use:           "tr",
		Short:         "Local-first Trade Republic portfolio and research CLI",
		Long:          "Normalize Trade Republic exports and statements into SQLite without exposing a live order endpoint.",
		Version:       Version,
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			if f.Agent {
				f.JSON = true
				f.NoInput = true
			}
			return nil
		},
	}
	persistent := root.PersistentFlags()
	persistent.BoolVar(&f.JSON, "json", false, "write versioned JSON to stdout")
	persistent.BoolVar(&f.Agent, "agent", false, "machine mode (implies --json and --no-input)")
	persistent.BoolVar(&f.Quiet, "quiet", false, "suppress non-error human output")
	persistent.BoolVar(&f.NoInput, "no-input", false, "never prompt for input")
	persistent.StringVar(&f.Config, "config", "", "configuration file")
	persistent.StringVar(&f.Database, "db", "", "SQLite database path")
	persistent.DurationVar(&f.Timeout, "timeout", 5*time.Minute, "command timeout")

	root.AddCommand(
		versionCmd(f), configCmd(f), doctorCmd(f), authCmd(f), syncCmd(f),
		portfolioCmd(f), positionCmd(f), allocationCmd(f), reportCmd(f),
		importCmd(f), searchCmd(f), researchCmd(f), compareCmd(f), newsCmd(f),
		orderCmd(f),
	)
	return root
}

func versionCmd(f *flags) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return emit(cmd, f, envelope{"version": 1, "cli_version": Version}, fmt.Sprintf("tr %s", Version))
		},
	}
}
