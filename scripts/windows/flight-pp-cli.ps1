param([Parameter(ValueFromRemainingArguments = $true)][string[]] $CliArgs)
& "$PSScriptRoot\Invoke-WslCli.ps1" -CommandName "flight-pp-cli" @CliArgs
exit $LASTEXITCODE
