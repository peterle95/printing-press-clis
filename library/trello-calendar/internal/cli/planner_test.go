package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"trello-calendar-pp-cli/internal/config"
	"trello-calendar-pp-cli/internal/googlecalendar"
	"trello-calendar-pp-cli/internal/scheduling"
	"trello-calendar-pp-cli/internal/workflow"

	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

func TestPlannerJSONIsValid(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	flags := &rootFlags{asJSON: true}
	value := map[string]any{"command": "preview", "result": workflow.PlanResult{Plan: scheduling.Plan{Timezone: "Europe/Berlin"}}}
	if err := flags.printJSON(cmd, value); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON %q: %v", out.String(), err)
	}
	if decoded["command"] != "preview" {
		t.Fatalf("unexpected JSON: %#v", decoded)
	}
}

func TestConfirmation(t *testing.T) {
	for input, want := range map[string]bool{"yes\n": true, "Y\n": true, "no\n": false, "\n": false} {
		var out bytes.Buffer
		got, err := confirm(bytes.NewBufferString(input), &out)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("confirm(%q)=%v want=%v", input, got, want)
		}
	}
}

func TestPlannerMCPAnnotations(t *testing.T) {
	root := RootCmd()
	for _, name := range []string{"cards", "preview", "schedule"} {
		cmd, _, err := root.Find([]string{name})
		if err != nil {
			t.Fatal(err)
		}
		if name != "schedule" && cmd.Annotations["mcp:read-only"] != "true" {
			t.Fatalf("%s must be read-only", name)
		}
		if name == "schedule" && cmd.Annotations["mcp:read-only"] == "true" {
			t.Fatal("schedule must not be read-only")
		}
	}
}

func TestScheduleDryRunMatchesPreviewAndPerformsNoWrites(t *testing.T) {
	var writes int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writes++
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/lists/list":
			w.Write([]byte(`{"id":"list","name":"Doing","idBoard":"board","closed":false}`))
		case r.URL.Path == "/boards/board":
			w.Write([]byte(`{"id":"board","name":"Planner","closed":false}`))
		case r.URL.Path == "/lists/list/cards":
			w.Write([]byte(`[{"id":"card","name":"Finish auth","url":"https://trello.com/c/card","pos":1,"closed":false}]`))
		case strings.Contains(r.URL.Path, "/events/tcc"):
			w.WriteHeader(http.StatusNotFound)
		case strings.HasSuffix(r.URL.Path, "/events"):
			w.Write([]byte(`{"items":[]}`))
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	configData := "timezone = \"Europe/Berlin\"\ntrello_board_id = \"board\"\ntrello_list_id = \"list\"\ngoogle_calendar_id = \"primary\"\n"
	if err := os.WriteFile(configPath, []byte(configData), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRELLO_API_KEY", "key")
	t.Setenv("TRELLO_TOKEN", "token")
	t.Setenv("GOOGLE_CLIENT_ID", "client")
	t.Setenv("GOOGLE_CLIENT_SECRET", "secret")
	t.Setenv("GOOGLE_REDIRECT_URI", "http://127.0.0.1:8765/oauth2/callback")
	t.Setenv("TRELLO_CALENDAR_BASE_URL", server.URL)
	t.Setenv("TRELLO_CALENDAR_GOOGLE_BASE_URL", server.URL)
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	store := googlecalendar.TokenStore{Path: cfg.TokenPath()}
	if err := store.Save(&oauth2.Token{AccessToken: "access", RefreshToken: "refresh", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}

	preview := executePlannerCommand(t, "--config", configPath, "--json", "preview")
	dryRun := executePlannerCommand(t, "--config", configPath, "--json", "--dry-run", "schedule")
	if writes != 0 {
		t.Fatalf("dry-run issued %d write request(s)", writes)
	}
	after, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("dry-run modified the token file")
	}
	previewResult := preview["result"].(map[string]any)
	schedulePlan := dryRun["plan"].(map[string]any)
	if !reflect.DeepEqual(previewResult["plan"], schedulePlan["plan"]) {
		t.Fatalf("preview and dry-run plans differ\npreview=%#v\ndry=%#v", previewResult["plan"], schedulePlan["plan"])
	}
}

func executePlannerCommand(t *testing.T, args ...string) map[string]any {
	t.Helper()
	root := RootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetIn(bytes.NewBuffer(nil))
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("command %v failed: %v stderr=%s stdout=%s", args, err, stderr.String(), stdout.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	return decoded
}
