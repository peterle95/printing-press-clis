package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"transit-pp-cli/internal/cache"
	"transit-pp-cli/internal/config"
	"transit-pp-cli/internal/provider/vbb"
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
	debug    bool
	yes      bool
}

type app struct {
	flags    *rootFlags
	config   config.Config
	cacheDir string
	cache    *cache.Store
	client   *vbb.Client
}

func Execute() error {
	return RootCmd().Execute()
}

func RootCmd() *cobra.Command {
	flags := &rootFlags{}
	root := &cobra.Command{
		Use:           "transit-pp-cli",
		Short:         "Real-time Berlin VBB/BVG departures, routes, leave times, and vehicle radar",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	root.SetVersionTemplate("transit-pp-cli {{ .Version }}\n")
	root.PersistentFlags().BoolVar(&flags.asJSON, "json", false, "Output stable JSON")
	root.PersistentFlags().BoolVar(&flags.agent, "agent", false, "Agent-friendly output (same as --json)")
	root.PersistentFlags().StringVar(&flags.config, "config", "", "Config file path")
	root.PersistentFlags().StringVar(&flags.cacheDir, "cache-dir", "", "Cache directory")
	root.PersistentFlags().DurationVar(&flags.timeout, "timeout", 10*time.Second, "HTTP timeout")
	root.PersistentFlags().BoolVar(&flags.noCache, "no-cache", false, "Bypass local response cache")
	root.PersistentFlags().BoolVar(&flags.debug, "debug", false, "Print request URLs and raw provider errors to stderr")
	root.PersistentFlags().BoolVar(&flags.yes, "yes", false, "Accept the first provider match when confirmation would be needed")

	root.AddCommand(newConfigCmd(flags))
	root.AddCommand(newNearbyCmd(flags))
	root.AddCommand(newBoardCmd(flags))
	root.AddCommand(newWatchCmd(flags))
	root.AddCommand(newRouteCmd(flags))
	root.AddCommand(newLeaveCmd(flags))
	root.AddCommand(newRadarCmd(flags))
	root.AddCommand(newTripCmd(flags))
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
	client := vbb.New(cfg.BaseURL, flags.timeout)
	client.Debug = flags.debug
	return &app{
		flags:    flags,
		config:   cfg,
		cacheDir: cacheDir,
		cache:    cache.New(cacheDir),
		client:   client,
	}, nil
}

func commandContext(flags *rootFlags) (context.Context, context.CancelFunc) {
	timeout := flags.timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return context.WithTimeout(context.Background(), timeout)
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func wantsJSON(flags *rootFlags) bool {
	return flags.asJSON || flags.agent
}

func outputValue(flags *rootFlags, v any, text func() error) error {
	if wantsJSON(flags) {
		return printJSON(v)
	}
	if text != nil {
		return text()
	}
	fmt.Println(v)
	return nil
}
