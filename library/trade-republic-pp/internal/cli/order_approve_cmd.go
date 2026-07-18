package cli

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"trade-republic-pp-cli/internal/execution"
)

func orderApproveCmd(f *flags) *cobra.Command {
	var challenge, approver string
	cmd := &cobra.Command{
		Use:   "approve <preview-id>",
		Short: "Record an exact preview-bound typed approval",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(approver) == "" {
				return fmt.Errorf("--approver is required")
			}
			ctx, cancel := commandContext(cmd.Context(), f.Timeout)
			defer cancel()
			database, cfg, closeStore, err := openStore(ctx, f)
			if err != nil {
				return err
			}
			defer closeStore()
			stored, err := database.LoadPreview(ctx, args[0])
			if err != nil {
				return err
			}
			provided := challenge
			if provided == "" {
				if f.NoInput {
					return fmt.Errorf("interactive typed approval is disabled; provide --challenge exactly")
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "Type this exact preview-bound phrase:\n%s\n> ", stored.Preview.ApprovalChallenge)
				line, readErr := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
				if readErr != nil && strings.TrimSpace(line) == "" {
					return readErr
				}
				provided = strings.TrimSpace(line)
			}
			engine, err := execution.NewEngine(database)
			if err != nil {
				return err
			}
			approval, err := engine.Approve(ctx, execution.ApprovalRequest{
				PreviewID: args[0], TypedChallenge: provided, Approver: approver, Now: time.Now().UTC(), Policy: executionPolicy(cfg),
			})
			if err != nil {
				return err
			}
			return emit(cmd, f, envelope{"version": 1, "paper_only": true, "approval": approval}, fmt.Sprintf("approved paper preview %s as %s", approval.PreviewID, approval.Approver))
		},
	}
	cmd.Flags().StringVar(&challenge, "challenge", "", "exact preview-bound challenge (prefer interactive input)")
	cmd.Flags().StringVar(&approver, "approver", "", "operator identity recorded in the local audit chain")
	return cmd
}
