package googlecalendar

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestTokenStoreAtomicPermissionsAndRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	store := TokenStore{Path: filepath.Join(dir, "google-token.json")}
	want := &oauth2.Token{AccessToken: "access", RefreshToken: "refresh", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour).Truncate(time.Second)}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	if err := store.PermissionStatus(); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken || !got.Expiry.Equal(want.Expiry) {
		t.Fatalf("token mismatch: %#v", got)
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(store.Path)
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("mode=%04o", info.Mode().Perm())
		}
	}
}

func TestTokenStoreRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte(`{"access_token":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "token")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := (TokenStore{Path: link}).Load(); err == nil {
		t.Fatal("expected symlink rejection")
	}
}
