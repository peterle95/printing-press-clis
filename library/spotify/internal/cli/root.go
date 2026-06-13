package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"spotify-pp-cli/internal/auth"
	"spotify-pp-cli/internal/client"
	"spotify-pp-cli/internal/config"
	"spotify-pp-cli/internal/store"
)

const (
	ExitOK           = 0
	ExitUsage        = 2
	ExitRuntimeError = 1
)

var version = "0.1.0"

type rootFlags struct {
	asJSON     bool
	configPath string
	dbPath     string
	yes        bool
	timeout    time.Duration
}

type app struct {
	Config     config.Config
	ConfigPath string
	DBPath     string
	Store      *store.Store
	Auth       *auth.Manager
	Client     *client.Client
}

func Execute() error {
	cmd := RootCmd()
	return cmd.Execute()
}

func RootCmd() *cobra.Command {
	flags := &rootFlags{}
	root := &cobra.Command{
		Use:           "spotify-pp-cli",
		Short:         "Organize a personal Spotify library with the official Spotify Web API",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	root.SetVersionTemplate("spotify-pp-cli {{ .Version }}\n")
	root.PersistentFlags().BoolVar(&flags.asJSON, "json", false, "Output machine-readable JSON")
	root.PersistentFlags().StringVar(&flags.configPath, "config", "", "Config file path")
	root.PersistentFlags().StringVar(&flags.dbPath, "db", "", "SQLite database path")
	root.PersistentFlags().BoolVar(&flags.yes, "yes", false, "Skip interactive confirmation prompts")
	root.PersistentFlags().DurationVar(&flags.timeout, "timeout", 30*time.Second, "Request timeout")

	root.AddCommand(newAuthCmd(flags))
	root.AddCommand(newConfigCmd(flags))
	root.AddCommand(newRulesCmd(flags))
	root.AddCommand(newLikedCmd(flags))
	root.AddCommand(newClassifyCmd(flags))
	root.AddCommand(newPlaylistsCmd(flags))
	root.AddCommand(newPlaylistCmd(flags))
	root.AddCommand(newTrackCmd(flags))
	root.AddCommand(newReviewCmd(flags))
	root.AddCommand(newOpsCmd(flags))
	return root
}

func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	return ExitRuntimeError
}

func loadApp(flags *rootFlags) (*app, func(), error) {
	cfg, cfgPath, err := config.Load(flags.configPath)
	if err != nil {
		return nil, nil, err
	}
	dbPath := flags.dbPath
	if dbPath == "" {
		dbPath, err = config.DBPath()
		if err != nil {
			return nil, nil, err
		}
	}
	st, err := store.Open(dbPath)
	if err != nil {
		return nil, nil, err
	}
	tokenStore, err := auth.NewDefaultTokenStore()
	if err != nil {
		_ = st.Close()
		return nil, nil, err
	}
	manager := auth.NewManager(cfg, tokenStore)
	manager.HTTP.Timeout = flags.timeout
	api := client.New(manager)
	api.HTTP.Timeout = flags.timeout
	cleanup := func() {
		_ = st.Close()
	}
	return &app{
		Config:     cfg,
		ConfigPath: cfgPath,
		DBPath:     dbPath,
		Store:      st,
		Auth:       manager,
		Client:     api,
	}, cleanup, nil
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
	if flags.asJSON {
		return printJSON(v)
	}
	if text != nil {
		return text()
	}
	fmt.Println(v)
	return nil
}
