package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"spotify-pp-cli/internal/config"
	"spotify-pp-cli/internal/rules"
)

func newRulesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "rules", Short: "Manage genre routing rules"}
	cmd.AddCommand(newRulesInitCmd(flags))
	cmd.AddCommand(newRulesPathCmd(flags))
	cmd.AddCommand(newRulesValidateCmd(flags))
	cmd.AddCommand(newRulesExplainCmd(flags))
	return cmd
}

func newRulesInitCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create the editable rules YAML if missing",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.RulesPath()
			if err != nil {
				return err
			}
			created, err := rules.Init(path)
			if err != nil {
				return err
			}
			return outputValue(flags, map[string]any{"path": path, "created": created}, func() error {
				if created {
					fmt.Printf("Created rules file: %s\n", path)
				} else {
					fmt.Printf("Rules file already exists: %s\n", path)
				}
				return nil
			})
		},
	}
}

func newRulesPathCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the rules YAML path",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.RulesPath()
			if err != nil {
				return err
			}
			return outputValue(flags, map[string]string{"path": path}, func() error {
				fmt.Println(path)
				return nil
			})
		},
	}
}

func newRulesValidateCmd(flags *rootFlags) *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate rules YAML syntax and routing targets",
		RunE: func(cmd *cobra.Command, args []string) error {
			rf, usedPath, err := loadRulesFile(path)
			if err != nil {
				return err
			}
			errs := rules.Validate(rf)
			payload := map[string]any{"path": usedPath, "valid": len(errs) == 0, "errors": errs}
			if len(errs) > 0 {
				return outputValue(flags, payload, func() error {
					for _, err := range errs {
						fmt.Printf("Error: %v\n", err)
					}
					return nil
				})
			}
			return outputValue(flags, payload, func() error {
				fmt.Printf("Rules OK: %s\n", usedPath)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&path, "rules", "", "Rules file path")
	return cmd
}

func newRulesExplainCmd(flags *rootFlags) *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use:   "explain <track-url-or-id-or-query>",
		Short: "Explain how one track would be classified",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, cleanup, err := loadApp(flags)
			if err != nil {
				return err
			}
			defer cleanup()
			ctx, cancel := commandContext(flags)
			defer cancel()
			rf, _, err := loadRulesFile(path)
			if err != nil {
				return err
			}
			track, err := resolveTrack(ctx, app, args[0])
			if err != nil {
				return err
			}
			result, err := classifyTrack(ctx, app.Store, rf, track)
			if err != nil {
				return err
			}
			return printClassification(flags, result)
		},
	}
	cmd.Flags().StringVar(&path, "rules", "", "Rules file path")
	return cmd
}
