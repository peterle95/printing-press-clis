package cli

import (
	"github.com/spf13/cobra"
)

func orderCmd(f *flags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "order",
		Short: "Create auditable paper-order artifacts; live submission is not implemented",
		Long:  "Preview, approve, and export deterministic paper-order artifacts. This binary has no broker order submission endpoint.",
	}
	cmd.AddCommand(orderPreviewCmd(f), orderApproveCmd(f), orderExportCmd(f))
	return cmd
}
