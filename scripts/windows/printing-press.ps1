param([Parameter(ValueFromRemainingArguments = $true)][string[]] $PrintingPressArgs)

$ErrorActionPreference = "Stop"
$Distro = "Ubuntu"
$PathFlags = @("--dir", "--har", "--ledger", "--output", "--plan", "--research-dir", "--spec", "--traffic-analysis")

function Convert-ToWslPathIfLocal([string] $Value) {
  if ([string]::IsNullOrWhiteSpace($Value) -or $Value -match "^[a-zA-Z][a-zA-Z0-9+.-]*://" -or $Value -like "~/*" -or $Value -like "/*") {
    return $Value
  }
  if (Test-Path -LiteralPath $Value) {
    $Value = (Resolve-Path -LiteralPath $Value).Path
  }
  if ($Value -match "^[A-Za-z]:[\\/]" -or $Value -match "^\\\\") {
    return (wsl.exe -d $Distro -- wslpath -a $Value).Trim()
  }
  return $Value
}

$Converted = [System.Collections.Generic.List[string]]::new()
for ($i = 0; $i -lt $PrintingPressArgs.Count; $i++) {
  $Arg = $PrintingPressArgs[$i]
  $Handled = $false
  foreach ($Flag in $PathFlags) {
    if ($Arg.StartsWith("$Flag=", [System.StringComparison]::Ordinal)) {
      $Converted.Add("$Flag=$(Convert-ToWslPathIfLocal $Arg.Substring($Flag.Length + 1))")
      $Handled = $true
      break
    }
  }
  if ($Handled) { continue }
  $Converted.Add($Arg)
  if ($PathFlags -contains $Arg -and ($i + 1) -lt $PrintingPressArgs.Count) {
    $i++
    $Converted.Add((Convert-ToWslPathIfLocal $PrintingPressArgs[$i]))
  }
}

& "$PSScriptRoot\Invoke-WslCli.ps1" -CommandName "printing-press" @Converted
exit $LASTEXITCODE
