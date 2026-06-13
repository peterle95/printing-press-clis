param([Parameter(ValueFromRemainingArguments = $true)][string[]] $CliArgs)
& "$PSScriptRoot\Invoke-WslCli.ps1" -CommandName "spotify-pp-cli" @CliArgs
exit $LASTEXITCODE
