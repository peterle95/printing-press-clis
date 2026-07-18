package cli

import (
	"io"

	"trade-republic-pp-cli/config"
	"trade-republic-pp-cli/providers/traderepublic"
)

func newTradeRepublicAdapter(cfg config.Config, stdin io.Reader, stdout, stderr io.Writer) (*traderepublic.Adapter, error) {
	return traderepublic.New(traderepublic.Options{
		Command:            cfg.PytrCommand,
		PDFToTextCommand:   cfg.PDFToTextCommand,
		StagingDirectory:   cfg.StagingDirectory,
		DocumentsDirectory: cfg.DocumentsDirectory,
		AccountTimezone:    cfg.AccountTimezone,
		BaseCurrency:       cfg.BaseCurrency,
		Timeout:            cfg.PytrTimeout.Duration(),
		Runner:             traderepublic.ExecRunner{},
		Stdin:              stdin,
		Stdout:             stdout,
		Stderr:             stderr,
	})
}
