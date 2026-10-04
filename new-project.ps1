param (
    [Parameter(Position=0)]
    [string]$ProjectName,

    [Parameter(Position=1)]
    [string]$Port,

    [Parameter(Position=2)]
    [string]$DatabaseDriver = "sqlite"
)

$ErrorActionPreference = "Stop"

Write-Host "==================================================================" -ForegroundColor Cyan
Write-Host "[TGo Booster] Instant New Project Creator" -ForegroundColor Cyan
Write-Host "==================================================================" -ForegroundColor Cyan

if (-not $ProjectName) {
    $ProjectName = Read-Host "Masukkan nama proyek baru (contoh: cms-portal, tokoku, erp-app)"
}

$ProjectName = $ProjectName.Trim()
if (-not $ProjectName) {
    Write-Host "[Error] Nama proyek tidak boleh kosong!" -ForegroundColor Red
    exit 1
}

$boosterExe = Join-Path $PSScriptRoot "booster.exe"
if (-not (Test-Path $boosterExe)) {
    Write-Host "[Build] Mengompilasi booster CLI..." -ForegroundColor Yellow
    & go build -o $boosterExe ./cmd/booster
}

$cmdArgs = @("new", $ProjectName, "--db", $DatabaseDriver)
if ($Port) {
    $cmdArgs += @("--port", $Port)
}

& $boosterExe $cmdArgs