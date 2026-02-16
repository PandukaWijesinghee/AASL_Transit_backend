param(
    [string]$DatabaseUrl = $env:DATABASE_URL
)

if (-not $DatabaseUrl -or $DatabaseUrl -eq "") {
    Write-Error "DATABASE_URL is not set. Provide -DatabaseUrl or set the env var."
    Write-Host "Example: .\init_database.ps1 -DatabaseUrl 'postgresql://user:password@host:port/dbname'"
    exit 1
}

$scriptPath = Join-Path $PSScriptRoot "01_init_database_schema.sql"

if (-not (Test-Path $scriptPath)) {
    Write-Error "Schema file not found: $scriptPath"
    exit 1
}

Write-Host "================================================"
Write-Host "  Database Schema Initialization"
Write-Host "================================================"
Write-Host ""
Write-Host "Target Database: $($DatabaseUrl -replace 'postgresql://[^@]+@', 'postgresql://***@')"
Write-Host "Schema File: $scriptPath"
Write-Host ""
Write-Host "⚠️  WARNING: This will create all tables, types, and constraints."
Write-Host "⚠️  If tables already exist, you may need to drop them first."
Write-Host ""

$confirm = Read-Host "Do you want to proceed? (yes/no)"
if ($confirm -ne "yes") {
    Write-Host "Aborted by user."
    exit 0
}

Write-Host ""
Write-Host "Initializing database schema..."

# Run psql with error stop; expects psql in PATH
& psql $DatabaseUrl -v ON_ERROR_STOP=1 -f $scriptPath

if ($LASTEXITCODE -ne 0) {
    Write-Error "psql failed with exit code $LASTEXITCODE"
    Write-Host ""
    Write-Host "Common issues:"
    Write-Host "  1. psql not installed or not in PATH"
    Write-Host "  2. Database credentials incorrect"
    Write-Host "  3. Database already has some tables (run clear_all_data.ps1 first)"
    Write-Host "  4. Schema contains syntax errors"
    exit $LASTEXITCODE
}

Write-Host ""
Write-Host "✅ Database schema initialized successfully!"
Write-Host ""
Write-Host "Next steps:"
Write-Host "  1. Verify tables were created: SELECT tablename FROM pg_tables WHERE schemaname='public';"
Write-Host "  2. Deploy/restart your backend application"
Write-Host "  3. Test the lounge booking creation"
