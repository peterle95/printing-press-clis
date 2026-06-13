package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"flight-pp-cli/internal/cache"
	"flight-pp-cli/internal/config"
)

const (
	ExitOK           = 0
	ExitRuntimeError = 1
)

var version = "0.1.0"

type rootFlags struct {
	asJSON   bool
	agent    bool
	config   string
	cacheDir string
	timeout  time.Duration
	noCache  bool
}

type app struct {
	flags    *rootFlags
	config   config.Config
	cacheDir string
	cache    *cache.Store
}

func Execute() error {
	return RootCmd().Execute()
}

func RootCmd() *cobra.Command {
	flags := &rootFlags{}
	root := &cobra.Command{
		Use:           "flight-pp-cli",
		Short:         "Compare flight prices with official APIs, affiliate APIs, and safe deep links",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	root.SetVersionTemplate("flight-pp-cli {{ .Version }}\n")
	root.PersistentFlags().BoolVar(&flags.asJSON, "json", false, "Output stable JSON")
	root.PersistentFlags().BoolVar(&flags.agent, "agent", false, "Agent-friendly output (same as --json)")
	root.PersistentFlags().StringVar(&flags.config, "config", "", "Config file path")
	root.PersistentFlags().StringVar(&flags.cacheDir, "cache-dir", "", "Cache directory")
	root.PersistentFlags().DurationVar(&flags.timeout, "timeout", 30*time.Second, "Network timeout")
	root.PersistentFlags().BoolVar(&flags.noCache, "no-cache", false, "Bypass provider response cache")

	root.AddCommand(newConfigCmd(flags))
	root.AddCommand(newProvidersCmd(flags))
	root.AddCommand(newSearchCmd(flags))
	root.AddCommand(newCheapestCmd(flags))
	root.AddCommand(newWatchCmd(flags))
	root.AddCommand(newCacheCmd(flags))
	return root
}

func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	return ExitRuntimeError
}

func loadApp(flags *rootFlags) (*app, error) {
	cfg, err := config.Load(flags.config)
	if err != nil {
		return nil, err
	}
	cacheDir := flags.cacheDir
	if cacheDir == "" {
		cacheDir, err = config.CacheDir()
		if err != nil {
			return nil, err
		}
	}
	ttl := time.Duration(cfg.Defaults.CacheTTLMinutes) * time.Minute
	if cfg.Defaults.CacheTTLMinutes < 0 {
		ttl = 0
	}
	return &app{
		flags:    flags,
		config:   cfg,
		cacheDir: cacheDir,
		cache:    cache.New(cacheDir, ttl),
	}, nil
}

func commandContext(flags *rootFlags) (context.Context, context.CancelFunc) {
	timeout := flags.timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return context.WithTimeout(context.Background(), timeout)
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func outputValue(flags *rootFlags, v any, text func() error) error {
	if flags.asJSON || flags.agent {
		return printJSON(v)
	}
	if text != nil {
		return text()
	}
	fmt.Println(v)
	return nil
}
