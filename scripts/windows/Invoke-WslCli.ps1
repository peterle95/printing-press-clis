param(
  [Parameter(Mandatory = $true)]
  [string] $CommandName,

  [Parameter(ValueFromRemainingArguments = $true)]
  [string[]] $CliArgs
)

$ErrorActionPreference = "Stop"
$Distro = "Ubuntu"
$WslHome = (& wsl.exe -d $Distro -- sh -lc 'printf "%s" "$HOME"').Trim()
if ([string]::IsNullOrWhiteSpace($WslHome)) {
  throw "Could not discover the WSL home directory for $Distro."
}
$WslPath = "$WslHome/.local/go/bin:$WslHome/go/bin:$WslHome/.local/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

& wsl.exe -d $Distro -- env "PATH=$WslPath" $CommandName @CliArgs
exit $LASTEXITCODE
