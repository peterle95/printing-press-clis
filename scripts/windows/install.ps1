$ErrorActionPreference = "Stop"

$Destination = Join-Path $env:LOCALAPPDATA "Programs\printing-press\bin"
New-Item -ItemType Directory -Force -Path $Destination | Out-Null

$Files = @(
  "Invoke-WslCli.ps1",
  "printing-press.ps1",
  "flight-pp-cli.ps1",
  "transit-pp-cli.ps1",
  "spotify-pp-cli.ps1"
)

foreach ($File in $Files) {
  Copy-Item -LiteralPath (Join-Path $PSScriptRoot $File) -Destination (Join-Path $Destination $File) -Force
}

Remove-Item -LiteralPath (Join-Path $Destination "messaging-pp-cli.ps1") -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath (Join-Path $Destination "messaging-pp-cli.exe") -Force -ErrorAction SilentlyContinue

Write-Host "Installed PowerShell launchers in $Destination"
