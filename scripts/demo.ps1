param(
    [string]$CopyCheck = (Join-Path $PSScriptRoot '..\dist\copycheck.exe')
)

$ErrorActionPreference = 'Stop'
$CopyCheck = (Resolve-Path -LiteralPath $CopyCheck).Path
$demoTemp = (Resolve-Path -LiteralPath ([IO.Path]::GetTempPath())).Path
$demoRoot = Join-Path $demoTemp ("copycheck-demo-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $demoRoot | Out-Null

function Run-CopyCheck {
    param(
        [int]$ExpectedExit,
        [string[]]$Arguments
    )
    Write-Host ('PS> copycheck.exe ' + ($Arguments -join ' '))
    & $CopyCheck @Arguments
    $actualExit = $LASTEXITCODE
    Write-Host "Код выхода: $actualExit"
    Write-Host ''
    if ($actualExit -ne $ExpectedExit) {
        throw "Ожидался код $ExpectedExit, получен $actualExit."
    }
}

Push-Location -LiteralPath $demoRoot
try {
    New-Item -ItemType Directory -Path 'source\docs','copy' -Force | Out-Null
    Set-Content -LiteralPath 'source\docs\receipt.txt' -Value 'Example receipt' -Encoding UTF8
    Set-Content -LiteralPath 'source\docs\budget.txt' -Value 'Total: 4500' -Encoding UTF8
    Set-Content -LiteralPath 'source\docs\ticket.txt' -Value 'Example ticket' -Encoding UTF8
    Set-Content -LiteralPath 'source\notes.txt' -Value 'Example notes' -Encoding UTF8

    Run-CopyCheck -ExpectedExit 0 -Arguments @('snapshot','source','--out','snapshot.json')
    Copy-Item -Path 'source\*' -Destination 'copy' -Recurse
    Run-CopyCheck -ExpectedExit 0 -Arguments @('verify','copy','--manifest','snapshot.json')

    Rename-Item -LiteralPath 'copy\docs\receipt.txt' -NewName 'receipt-final.txt'
    Set-Content -LiteralPath 'copy\docs\budget.txt' -Value 'Total: 4900' -Encoding UTF8
    Remove-Item -LiteralPath 'copy\docs\ticket.txt'
    Set-Content -LiteralPath 'copy\readme.txt' -Value 'Added after copying' -Encoding UTF8
    Run-CopyCheck -ExpectedExit 1 -Arguments @('verify','copy','--manifest','snapshot.json')
} finally {
    Pop-Location
    $resolvedDemo = (Resolve-Path -LiteralPath $demoRoot).Path
    $parent = Split-Path -Parent $resolvedDemo
    $leaf = Split-Path -Leaf $resolvedDemo
    if ($parent -eq $demoTemp.TrimEnd('\','/') -and $leaf -match '^copycheck-demo-[a-f0-9]{32}$') {
        Remove-Item -LiteralPath $resolvedDemo -Recurse -Force
    }
}
