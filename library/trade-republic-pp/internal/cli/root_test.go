package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"trade-republic-pp-cli/config"
	"trade-republic-pp-cli/internal/execution"
)

func TestOfflineSyncAndPaperOrderWorkflow(t *testing.T) {
	directory := t.TempDir()
	configPath := testConfig(t, directory)
	fixture, err := filepath.Abs(filepath.Join("..", "..", "testdata", "example.finance.json"))
	if err != nil {
		t.Fatal(err)
	}

	syncOutput, err := executeCommand(t, "--config", configPath, "sync", "--provider", "financejson", "--input", fixture, "--json")
	if err != nil {
		t.Fatalf("sync: %v\n%s", err, syncOutput)
	}

	previewOutput, err := executeCommand(t,
		"--config", configPath, "order", "preview", "--buy", "IE00B4L5Y983",
		"--amount", "100", "--limit-price", "100",
		"--nonce", "00112233445566778899aabbccddeeff", "--json",
	)
	if err != nil {
		t.Fatalf("preview: %v\n%s", err, previewOutput)
	}
	var previewEnvelope struct {
		Preview execution.Preview `json:"preview"`
	}
	if err := json.Unmarshal(previewOutput, &previewEnvelope); err != nil {
		t.Fatal(err)
	}
	preview := previewEnvelope.Preview
	if !preview.Decision.Allowed || preview.ID == "" || preview.ApprovalChallenge == "" {
		t.Fatalf("unexpected preview: %#v", preview)
	}

	approvalOutput, err := executeCommand(t,
		"--config", configPath, "order", "approve", preview.ID,
		"--approver", "integration-test", "--challenge", preview.ApprovalChallenge, "--json",
	)
	if err != nil {
		t.Fatalf("approve: %v\n%s", err, approvalOutput)
	}
	var approvalEnvelope struct {
		Approval execution.Approval `json:"approval"`
	}
	if err := json.Unmarshal(approvalOutput, &approvalEnvelope); err != nil {
		t.Fatal(err)
	}
	if approvalEnvelope.Approval.PreviewID != preview.ID {
		t.Fatalf("approval = %#v", approvalEnvelope.Approval)
	}

	exportOutput, err := executeCommand(t, "--config", configPath, "order", "export", preview.ID, "--format", "json")
	if err != nil {
		t.Fatalf("export: %v\n%s", err, exportOutput)
	}
	var manifest map[string]any
	if err := json.Unmarshal(exportOutput, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest["schema"] != "trpp.paper-order/v1" || manifest["live_submission_supported"] != false {
		t.Fatalf("manifest = %#v", manifest)
	}
}

func TestSyncDryRunDoesNotCreateDatabase(t *testing.T) {
	directory := t.TempDir()
	configPath := testConfig(t, directory)
	cfg, _, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := filepath.Abs(filepath.Join("..", "..", "testdata", "example.finance.json"))
	if err != nil {
		t.Fatal(err)
	}
	if output, err := executeCommand(t, "--config", configPath, "sync", "--provider", "financejson", "--input", fixture, "--dry-run", "--json"); err != nil {
		t.Fatalf("dry run: %v\n%s", err, output)
	}
	if _, err := os.Stat(cfg.DatabasePath); !os.IsNotExist(err) {
		t.Fatalf("dry run created database %s", cfg.DatabasePath)
	}
}

func TestCommandTreeHasNoLiveExecute(t *testing.T) {
	root := RootCmd()
	if command, _, err := root.Find([]string{"order", "execute"}); err == nil && command.Name() == "execute" {
		t.Fatal("live order execute command exists")
	}
	login, _, err := root.Find([]string{"auth", "login"})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"pin", "phone", "otp", "cookie", "waf-token"} {
		if login.Flags().Lookup(forbidden) != nil {
			t.Fatalf("auth login exposes forbidden --%s flag", forbidden)
		}
	}
}

func testConfig(t *testing.T, directory string) string {
	t.Helper()
	cfg, err := config.Default()
	if err != nil {
		t.Fatal(err)
	}
	cfg.DatabasePath = filepath.Join(directory, "private", "data.db")
	cfg.DocumentsDirectory = filepath.Join(directory, "documents")
	cfg.StagingDirectory = filepath.Join(directory, "staging")
	cfg.Risk.KillSwitch = false
	cfg.Risk.PaperTrading = true
	cfg.Risk.PermittedISINs = []string{"IE00B4L5Y983"}
	cfg.Risk.PriceMaxAge = config.Duration(365 * 24 * time.Hour)
	cfg.Risk.BalanceMaxAge = config.Duration(365 * 24 * time.Hour)
	path := filepath.Join(directory, "config.yaml")
	if _, err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	return path
}

func executeCommand(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()
	command := RootCmd()
	var output, diagnostics bytes.Buffer
	command.SetArgs(args)
	command.SetOut(&output)
	command.SetErr(&diagnostics)
	command.SetIn(bytes.NewBuffer(nil))
	err := command.Execute()
	if diagnostics.Len() > 0 {
		output.Write(diagnostics.Bytes())
	}
	return output.Bytes(), err
}
