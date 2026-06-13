package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"unicode/utf16"
)

func localSecretPrelude(accountName string) string {
	return `
$ErrorActionPreference = 'Stop'
$account = ` + powerShellSingleQuote(accountName) + `
if ([string]::IsNullOrWhiteSpace($account)) { throw 'missing account name' }
$safe = $account -replace '[^A-Za-z0-9_.-]', '_'
$root = Join-Path ([Environment]::GetFolderPath('LocalApplicationData')) 'PrintingPress\Mail\Secrets'
$path = Join-Path $root ("proton-" + $safe + ".dpapi")
`
}

func storeLocalSecret(accountName, password string) (string, error) {
	script := localSecretPrelude(accountName) + `
New-Item -ItemType Directory -Force -Path $root | Out-Null
$plain = [Console]::In.ReadToEnd().TrimEnd([char]13, [char]10)
if ([string]::IsNullOrEmpty($plain)) { throw 'empty password' }
$secure = ConvertTo-SecureString -String $plain -AsPlainText -Force
$encrypted = ConvertFrom-SecureString -SecureString $secure
Set-Content -LiteralPath $path -Value $encrypted -NoNewline -Encoding ascii
[Console]::Out.Write($path)
`
	out, err := runLocalSecretPowerShell(script, password)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func readLocalSecret(accountName string) (string, error) {
	script := localSecretPrelude(accountName) + `
if (!(Test-Path -LiteralPath $path)) { throw "secret not found: $path" }
$encrypted = Get-Content -LiteralPath $path -Raw
$secure = ConvertTo-SecureString -String $encrypted
$bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
try {
  [Console]::Out.Write([Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr))
} finally {
  if ($bstr -ne [IntPtr]::Zero) {
    [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
  }
}
`
	return runLocalSecretPowerShell(script, "")
}

func localSecretStatus(accountName string) (map[string]any, error) {
	script := localSecretPrelude(accountName) + `
$result = [ordered]@{
  path = $path
  stored = (Test-Path -LiteralPath $path)
}
$result | ConvertTo-Json -Compress
`
	out, err := runLocalSecretPowerShell(script, "")
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, fmt.Errorf("parsing local secret status: %w", err)
	}
	return result, nil
}

func deleteLocalSecret(accountName string) (map[string]any, error) {
	script := localSecretPrelude(accountName) + `
$existed = Test-Path -LiteralPath $path
if ($existed) { Remove-Item -LiteralPath $path -Force }
$result = [ordered]@{
  path = $path
  deleted = $existed
}
$result | ConvertTo-Json -Compress
`
	out, err := runLocalSecretPowerShell(script, "")
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, fmt.Errorf("parsing local secret delete status: %w", err)
	}
	return result, nil
}

func runLocalSecretPowerShell(script, stdin string) (string, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-EncodedCommand", encodePowerShell(script))
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = "is powershell.exe available from WSL?"
		}
		return "", fmt.Errorf("local encrypted secret store failed: %w: %s", err, detail)
	}
	return stdout.String(), nil
}

func encodePowerShell(script string) string {
	encoded := utf16.Encode([]rune(script))
	buf := make([]byte, len(encoded)*2)
	for i, value := range encoded {
		buf[i*2] = byte(value)
		buf[i*2+1] = byte(value >> 8)
	}
	return base64.StdEncoding.EncodeToString(buf)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func powerShellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
