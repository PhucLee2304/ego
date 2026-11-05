param(
    [Parameter(Mandatory = $true)][ValidateSet("reset-db", "seed")][string]$Action,
    [Parameter(Mandatory = $true)][string]$Service
)

$ErrorActionPreference = "Stop"
$databases = @{
    identity      = "ego-users-db"
    users         = "ego-users-db"
    knowledge     = "ego-knowledge-db"
    learning      = "ego-learning-db"
    notifications = "ego-notifications-db"
}

if ($Action -eq "seed") {
    $allowed = @("knowledge-topics", "learning")
    if ($Service -notin $allowed) { throw "Seed is only available for: $($allowed -join ', ')" }
    docker compose --profile seed run --rm --no-deps --build "$Service-seed"
    if ($LASTEXITCODE -ne 0) { throw "Seed failed for $Service" }
    exit
}

if (-not $databases.ContainsKey($Service)) { throw "Unsupported service: $Service" }
docker exec $databases[$Service] sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"'
if ($LASTEXITCODE -ne 0) { throw "Reset failed for $Service" }
$container = if ($Service -in @("users", "identity")) { "identity" } else { $Service }
docker compose restart $container