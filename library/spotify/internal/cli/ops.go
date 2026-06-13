package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"spotify-pp-cli/internal/organizer"
	"spotify-pp-cli/internal/ui"
)

func newOpsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "ops", Short: "Inspect and undo operation history"}
	cmd.AddCommand(newOpsListCmd(flags))
	cmd.AddCommand(newOpsShowCmd(flags))
	cmd.AddCommand(newOpsUndoCmd(flags))
	return cmd
}

func newOpsListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List recent operations",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, cleanup, err := loadApp(flags)
			if err != nil {
				return err
			}
			defer cleanup()
			ctx, cancel := commandContext(flags)
			defer cancel()
			ops, err := app.Store.Operations(ctx)
			if err != nil {
				return err
			}
			return outputValue(flags, ops, func() error {
				table := ui.NewTable(cmd.OutOrStdout())
				table.Row("ID", "Type", "Status", "Dry-run", "Created")
				for _, op := range ops {
					table.Row(op.ID, op.Type, op.Status, op.DryRun, op.CreatedAt)
				}
				return table.Flush()
			})
		},
	}
}

func newOpsShowCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "show <operation-id>",
		Short: "Show an operation and its items",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			app, cleanup, err := loadApp(flags)
			if err != nil {
				return err
			}
			defer cleanup()
			ctx, cancel := commandContext(flags)
			defer cancel()
			op, ok, err := app.Store.Operation(ctx, id)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("operation %d not found", id)
			}
			items, err := app.Store.OperationItems(ctx, id)
			if err != nil {
				return err
			}
			payload := map[string]any{"operation": op, "items": items}
			return outputValue(flags, payload, func() error {
				fmt.Printf("Operation %d: %s (%s)\n", op.ID, op.Type, op.Status)
				table := ui.NewTable(cmd.OutOrStdout())
				table.Row("Track", "Action", "Status", "Reason")
				for _, item := range items {
					table.Row(item.TrackID, item.Action, item.Status, item.Reason)
				}
				return table.Flush()
			})
		},
	}
}

func newOpsUndoCmd(flags *rootFlags) *cobra.Command {
	var dryRun bool
	var confirmFlag bool
	cmd := &cobra.Command{
		Use:   "undo <operation-id>",
		Short: "Undo a recorded operation where possible",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			app, cleanup, err := loadApp(flags)
			if err != nil {
				return err
			}
			defer cleanup()
			ctx, cancel := commandContext(flags)
			defer cancel()
			op, ok, err := app.Store.Operation(ctx, id)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("operation %d not found", id)
			}
			items, err := app.Store.OperationItems(ctx, id)
			if err != nil {
				return err
			}
			plan := organizer.BuildUndoPlan(op, items)
			if confirmFlag && !cmd.Flags().Changed("dry-run") {
				dryRun = false
			}
			if dryRun || !confirmFlag {
				return outputValue(flags, plan, func() error {
					fmt.Printf("Dry-run undo for operation %d\n", id)
					fmt.Printf("Would re-save %d tracks to Liked Songs.\n", len(plan.AddToLibrary))
					for playlistID, uris := range plan.RemovePlaylist {
						fmt.Printf("Would remove %d added tracks from playlist %s.\n", len(uris), playlistID)
					}
					for _, warning := range plan.Warnings {
						fmt.Printf("Warning: %s\n", warning)
					}
					return nil
				})
			}
			if err := confirm(flags, true, "undo spotify operation"); err != nil {
				return err
			}
			if len(plan.AddToLibrary) > 0 {
				if err := app.Client.SaveLibraryItems(ctx, plan.AddToLibrary); err != nil {
					return err
				}
			}
			for playlistID, uris := range plan.RemovePlaylist {
				if _, err := app.Client.RemovePlaylistItems(ctx, playlistID, uris); err != nil {
					return err
				}
			}
			fmt.Printf("Undo attempted for operation %d.\n", id)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", true, "Preview undo actions")
	cmd.Flags().BoolVar(&confirmFlag, "confirm", false, "Apply undo")
	return cmd
}
