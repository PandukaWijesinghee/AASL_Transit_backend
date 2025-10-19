param(
    [string]$DatabaseUrl = $env:DATABASE_URL
)

if (-not $DatabaseUrl -or $DatabaseUrl -eq "") {
    Write-Error "DATABASE_URL is not set. Provide -DatabaseUrl or set the env var."
    exit 1
}

$scriptPath = Join-Path $PSScriptRoot "clear_all_data.sql"

Write-Host "Clearing all data from application tables..."

# Run psql with error stop; expects psql in PATH
& psql $DatabaseUrl -v ON_ERROR_STOP=1 -f $scriptPath

if ($LASTEXITCODE -ne 0) {
    Write-Error "psql failed with exit code $LASTEXITCODE"
    exit $LASTEXITCODE
}

Write-Host "All data cleared successfully."