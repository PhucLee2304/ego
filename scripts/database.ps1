param(
    [Parameter(Mandatory = $true)][ValidateSet("reset-db", "seed")][string]$Action,
    [Parameter(Mandatory = $true)][string]$Service
)

$ErrorActionPreference = "Stop"
$databases = @{ identity = "users-db"; users = "users-db"; topics = "topics-db"; exams = "exams-db"; classrooms = "classrooms-db"; learning = "learning-db"; notifications = "notifications-db" }

if ($Action -eq "seed") {
    if ($Service -notin @("topics", "exams", "learning")) { throw "Seed is only available for topics, exams and learning" }
    docker compose --profile seed run --rm "$Service-seed"
    if ($LASTEXITCODE -ne 0) { throw "Seed failed for $Service" }
    exit
}

if (-not $databases.ContainsKey($Service)) { throw "Unsupported service: $Service" }
docker exec $databases[$Service] sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"'
if ($LASTEXITCODE -ne 0) { throw "Reset failed for $Service" }
$container = if ($Service -in @("users", "identity")) { "identity" } else { $Service }
docker compose restart $container
