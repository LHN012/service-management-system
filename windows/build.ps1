$ErrorActionPreference = 'Stop'

$WindowsRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$Frontend = Join-Path $WindowsRoot 'frontend'
$Output = Join-Path $WindowsRoot 'bin\service-management-system-windows.exe'

function Assert-NativeCommand([string]$Name) {
    if ($LASTEXITCODE -ne 0) {
        throw "$Name failed with exit code $LASTEXITCODE"
    }
}

Push-Location $Frontend
try {
    npm ci
    Assert-NativeCommand 'npm ci'
    npm run lint
    Assert-NativeCommand 'npm run lint'
    npm run build
    Assert-NativeCommand 'npm run build'
}
finally {
    Pop-Location
}

Push-Location $WindowsRoot
try {
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Output) | Out-Null
    go test ./...
    Assert-NativeCommand 'go test'
    go build -trimpath -tags 'desktop,production' -ldflags '-s -w -H windowsgui' -o $Output .
    Assert-NativeCommand 'go build'
}
finally {
    Pop-Location
}

Write-Host "Built $Output"
