package cli

import (
	"context"

	"trade-republic-pp-cli/config"
	store "trade-republic-pp-cli/storage/sqlite"
)

func openStore(ctx context.Context, f *flags) (*store.Store, config.Config, func(), error) {
	cfg, _, err := loadConfig(f)
	if err != nil {
		return nil, config.Config{}, nil, err
	}
	database, err := store.Open(ctx, cfg.DatabasePath)
	if err != nil {
		return nil, config.Config{}, nil, err
	}
	return database, cfg, func() { _ = database.Close() }, nil
}
