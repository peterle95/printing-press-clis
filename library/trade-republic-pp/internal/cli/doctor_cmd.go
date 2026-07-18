package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

type doctorCheck struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Required bool   `json:"required"`
	Detail   string `json:"detail"`
}

func doctorCmd(f *flags) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check configuration and optional adapter dependencies",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, configPath, err := loadConfig(f)
			if err != nil {
				return err
			}
			checks := []doctorCheck{{Name: "config", Status: "ok", Required: true, Detail: configPath}}
			if info, statErr := os.Stat(cfg.DatabasePath); statErr == nil {
				status := "ok"
				if !info.Mode().IsRegular() {
					status = "error"
				}
				checks = append(checks, doctorCheck{Name: "database", Status: status, Required: true, Detail: cfg.DatabasePath})
			} else if os.IsNotExist(statErr) {
				checks = append(checks, doctorCheck{Name: "database", Status: "not_created", Required: false, Detail: "created on first persisted import: " + cfg.DatabasePath})
			} else {
				checks = append(checks, doctorCheck{Name: "database", Status: "error", Required: true, Detail: statErr.Error()})
			}
			parent := filepath.Dir(cfg.DatabasePath)
			if info, statErr := os.Stat(parent); statErr == nil && info.IsDir() {
				checks = append(checks, doctorCheck{Name: "data_directory", Status: "ok", Required: true, Detail: parent})
			} else if os.IsNotExist(statErr) {
				checks = append(checks, doctorCheck{Name: "data_directory", Status: "not_created", Required: false, Detail: parent})
			} else if statErr != nil {
				checks = append(checks, doctorCheck{Name: "data_directory", Status: "error", Required: true, Detail: statErr.Error()})
			}
			adapter, adapterErr := newTradeRepublicAdapter(cfg, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
			if adapterErr != nil {
				checks = append(checks, doctorCheck{Name: "pytr", Status: "error", Required: false, Detail: adapterErr.Error()})
			} else {
				ctx, cancel := commandContext(cmd.Context(), f.Timeout)
				status := adapter.Status(ctx)
				cancel()
				state := "unavailable"
				if status.Available {
					state = "ok"
				}
				checks = append(checks, doctorCheck{Name: "pytr", Status: state, Required: false, Detail: status.Version + status.Detail})
			}
			pdfStatus, pdfDetail := "unavailable", cfg.PDFToTextCommand[0]
			if path, lookErr := exec.LookPath(cfg.PDFToTextCommand[0]); lookErr == nil {
				pdfStatus, pdfDetail = "ok", path
			}
			checks = append(checks, doctorCheck{Name: "pdftotext", Status: pdfStatus, Required: false, Detail: pdfDetail})
			checks = append(checks,
				doctorCheck{Name: "kill_switch", Status: safetyStatus(cfg.Risk.KillSwitch), Required: true, Detail: fmt.Sprintf("enabled=%t", cfg.Risk.KillSwitch)},
				doctorCheck{Name: "paper_trading", Status: safetyStatus(cfg.Risk.PaperTrading), Required: true, Detail: fmt.Sprintf("enabled=%t", cfg.Risk.PaperTrading)},
				doctorCheck{Name: "live_execution", Status: "not_implemented", Required: false, Detail: "the tr binary has no broker order submission endpoint"},
			)
			healthy := true
			for _, check := range checks {
				if check.Required && check.Status == "error" {
					healthy = false
				}
			}
			human := "doctor: ready for offline imports"
			if !healthy {
				human = "doctor: required checks failed"
			}
			return emit(cmd, f, envelope{"version": 1, "healthy": healthy, "checks": checks}, human)
		},
	}
}

func safetyStatus(enabled bool) string {
	if enabled {
		return "ok"
	}
	return "warning"
}
