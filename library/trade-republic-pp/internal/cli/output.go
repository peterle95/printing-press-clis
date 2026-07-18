package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

type envelope map[string]any

func emit(cmd *cobra.Command, f *flags, value any, human string) error {
	if f.JSON {
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value)
	}
	if f.Quiet {
		return nil
	}
	_, err := fmt.Fprintln(cmd.OutOrStdout(), human)
	return err
}

func emitJSONOnly(cmd *cobra.Command, value any) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
