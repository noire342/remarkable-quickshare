$ErrorActionPreference = 'Stop'
$go = Join-Path $PSScriptRoot '.tools/go/bin/go.exe'
if (-not (Test-Path -LiteralPath $go)) { $go = (Get-Command go -ErrorAction Stop).Source }
New-Item -ItemType Directory -Force (Join-Path $PSScriptRoot 'dist') | Out-Null
Push-Location (Join-Path $PSScriptRoot 'kindshare')
try {
    & $go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Tests failed' }
    $env:GOOS = 'linux'
    $env:GOARCH = 'arm'
    $env:GOARM = '7'
    $env:CGO_ENABLED = '0'
    & $go build -trimpath -ldflags='-s -w' -o ../dist/kindshare .
    if ($LASTEXITCODE -ne 0) { throw 'Build failed' }
} finally {
    Remove-Item Env:GOOS,Env:GOARCH,Env:GOARM,Env:CGO_ENABLED -ErrorAction SilentlyContinue
    Pop-Location
}
Get-FileHash -LiteralPath (Join-Path $PSScriptRoot 'dist/kindshare') -Algorithm SHA256
