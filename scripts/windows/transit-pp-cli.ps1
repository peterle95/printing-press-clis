param([Parameter(ValueFromRemainingArguments = $true)][string[]] $CliArgs)
& "$PSScriptRoot\Invoke-WslCli.ps1" -CommandName "transit-pp-cli" @CliArgs
exit $LASTEXITCODE
