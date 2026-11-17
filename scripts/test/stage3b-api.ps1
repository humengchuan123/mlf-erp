$ErrorActionPreference = 'Stop'

$root = Resolve-Path (Join-Path $PSScriptRoot '../..')
Set-Location $root
$env:APP_ENV = 'test'

function Wait-Docker {
    $path = 'C:\Program Files\Docker\Docker\Docker Desktop.exe'
    cmd /c "docker info >nul 2>&1"
    if ($LASTEXITCODE -ne 0 -and (Test-Path $path)) {
        Start-Process -FilePath $path
    }
    for ($i = 0; $i -lt 60; $i++) {
        cmd /c "docker info >nul 2>&1"
        if ($LASTEXITCODE -eq 0) {
            return
        }
        Start-Sleep -Seconds 2
    }
    throw 'Docker engine is not ready'
}

function Get-PostgresContainer {
    $id = docker compose -f deploy/compose/docker-compose.dev.yml ps -q postgres 2>$null
    if ($id) {
        return $id.Trim()
    }
    return $null
}

function Wait-Postgres {
    for ($i = 0; $i -lt 60; $i++) {
        $container = Get-PostgresContainer
        if ($container) {
            $status = docker inspect --format='{{.State.Health.Status}}' $container 2>$null
            if ($status -eq 'healthy') {
                return
            }
        }
        Start-Sleep -Seconds 2
    }
    throw 'PostgreSQL container is not healthy'
}

function Wait-Api {
    for ($i = 0; $i -lt 30; $i++) {
        try {
            $response = Invoke-RestMethod -Uri 'http://localhost:18080/readyz' -TimeoutSec 2
            if ($response.code -eq 'OK') {
                return
            }
        } catch {
        }
        Start-Sleep -Seconds 1
    }
    throw 'API is not ready on http://localhost:18080'
}

Wait-Docker

$postgresContainer = Get-PostgresContainer
$postgresStatus = $null
if ($postgresContainer) {
    $postgresStatus = docker inspect --format='{{.State.Health.Status}}' $postgresContainer 2>$null
}
if ($postgresStatus -ne 'healthy') {
    docker compose -f deploy/compose/docker-compose.dev.yml up -d postgres
}
Wait-Postgres

Write-Host 'Applying migrations and seed data...'
go run ./cmd/migrate -direction up
if ($LASTEXITCODE -ne 0) { throw 'migration failed' }
go run ./cmd/seed
if ($LASTEXITCODE -ne 0) { throw 'seed failed' }
go run ./cmd/demo
if ($LASTEXITCODE -ne 0) { throw 'demo seed failed' }

Write-Host 'Building API...'
New-Item -ItemType Directory -Force -Path dist | Out-Null
go build -o dist/api-stage3b.exe ./cmd/api
if ($LASTEXITCODE -ne 0) { throw 'api build failed' }

$env:HTTP_ADDR = ':18080'
$env:LOG_LEVEL = 'error'
$stdout = Join-Path $root 'dist/api-stage3b.log'
$stderr = Join-Path $root 'dist/api-stage3b.err'
$process = Start-Process -FilePath (Join-Path $root 'dist/api-stage3b.exe') `
    -WorkingDirectory $root -PassThru -NoNewWindow `
    -RedirectStandardOutput $stdout -RedirectStandardError $stderr

$testFailed = $false
try {
    Wait-Api
    Write-Host 'Running Stage 3B API integration tests...'
    $env:TEST_API_BASE_URL = 'http://localhost:18080'
    go test -tags=integration ./tests/api -run 'TestStage3B' -count=1 -v
    if ($LASTEXITCODE -ne 0) {
        $testFailed = $true
        throw 'Stage 3B API integration tests failed'
    }
    Write-Host 'Stage 3B API integration tests passed.'
} catch {
    $testFailed = $true
    Write-Host "Test run failed: $($_.Exception.Message)"
    if (Test-Path $stderr) {
        Write-Host '--- API stderr ---'
        Get-Content $stderr | Select-Object -Last 40
    }
    throw
} finally {
    if ($process -and -not $process.HasExited) {
        Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
    }
    Get-Process -Name api-stage3b -ErrorAction SilentlyContinue | Stop-Process -Force
    if (-not $testFailed) {
        Remove-Item -Force $stdout, $stderr, (Join-Path $root 'dist/api-stage3b.exe') -ErrorAction SilentlyContinue
    }
}
