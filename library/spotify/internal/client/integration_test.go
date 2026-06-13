package client_test

import (
	"context"
	"os"
	"testing"
	"time"

	"spotify-pp-cli/internal/auth"
	"spotify-pp-cli/internal/client"
	"spotify-pp-cli/internal/config"
)

func TestIntegrationCurrentUser(t *testing.T) {
	if os.Getenv("SPOTIFY_INTEGRATION_TESTS") != "1" {
		t.Skip("set SPOTIFY_INTEGRATION_TESTS=1 to run live Spotify tests")
	}
	cfg, _, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	tokenStore, err := auth.NewDefaultTokenStore()
	if err != nil {
		t.Fatal(err)
	}
	api := client.New(auth.NewManager(cfg, tokenStore))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	user, err := api.CurrentUser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if user.ID == "" {
		t.Fatal("current user id is empty")
	}
}
