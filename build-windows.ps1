$ErrorActionPreference = 'Stop'

$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$Frontend = Join-Path $Root 'windows\frontend'
$Output = Join-Path $Root 'bin\service-management-system-windows.exe'

Push-Location $Frontend
try {
    npm ci
    npm run lint
    npm run build
}
finally {
    Pop-Location
}

Push-Location $Root
try {
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Output) | Out-Null
    go test ./cmd/... ./internal/... ./windows
    go build -trimpath -tags 'desktop,production' -ldflags '-s -w -H windowsgui' -o $Output ./windows
}
finally {
    Pop-Location
}

Write-Host "Built $Output"
