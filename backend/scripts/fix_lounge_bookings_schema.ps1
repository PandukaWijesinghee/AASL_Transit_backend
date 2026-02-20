param(
    [string]$DatabaseUrl = $env:DATABASE_URL
)

if (-not $DatabaseUrl -or $DatabaseUrl -eq "") {
    Write-Error "DATABASE_URL is not set. Provide -DatabaseUrl or set the env var."
    Write-Host "Example: .\fix_lounge_bookings_schema.ps1 -DatabaseUrl 'postgresql://postgres:password@localhost:5432/smart_transit_dev'"
    exit 1
}

$scriptPath = Join-Path $PSScriptRoot "fix_lounge_bookings_schema.sql"

Write-Host "Applying lounge_bookings schema fix..."
Write-Host "This will rename the primary key column from 'lounge_booking_id' to 'id'"

# Run psql with error stop; expects psql in PATH
& psql $DatabaseUrl -v ON_ERROR_STOP=1 -f $scriptPath

if ($LASTEXITCODE -ne 0) {
    Write-Error "psql failed with exit code $LASTEXITCODE"
    exit $LASTEXITCODE
}

Write-Host ""
Write-Host "✅ Schema fix applied successfully!" -ForegroundColor Green
Write-Host "The lounge_bookings table now uses 'id' as the primary key column."
