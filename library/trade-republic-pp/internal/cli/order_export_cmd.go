package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"trade-republic-pp-cli/internal/execution"
)

func orderExportCmd(f *flags) *cobra.Command {
	var format, output string
	var force bool
	cmd := &cobra.Command{
		Use:   "export <preview-id>",
		Short: "Export an approved paper-order manifest without submitting it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.ToLower(format) != "json" {
				return fmt.Errorf("unsupported format %q (only json is available)", format)
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
			if stored.Approval == nil {
				return fmt.Errorf("preview %s has no recorded approval", args[0])
			}
			if err := execution.VerifyApprovedPreviewForExport(stored, executionPolicy(cfg), time.Now().UTC()); err != nil {
				return err
			}
			manifest := envelope{
				"schema": "trpp.paper-order/v1", "paper_only": true,
				"live_submission_supported": false, "exported_at": time.Now().UTC(),
				"preview": stored.Preview, "approval": stored.Approval,
			}
			if output == "" {
				return emitJSONOnly(cmd, manifest)
			}
			raw, err := json.MarshalIndent(manifest, "", "  ")
			if err != nil {
				return err
			}
			raw = append(raw, '\n')
			if err := writePrivateExport(output, raw, force); err != nil {
				return err
			}
			return emit(cmd, f, envelope{"version": 1, "path": output, "preview_id": args[0], "paper_only": true}, "wrote paper-only order manifest "+output)
		},
	}
	cmd.Flags().StringVar(&format, "format", "json", "export format (json)")
	cmd.Flags().StringVar(&output, "output", "", "private output file (defaults to stdout)")
	cmd.Flags().BoolVar(&force, "force", false, "replace an existing export file")
	return cmd
}

func writePrivateExport(path string, data []byte, force bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	flags := os.O_WRONLY | os.O_CREATE
	if force {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}
	file, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("export already exists at %s (use --force to replace it)", path)
		}
		return err
	}
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}
