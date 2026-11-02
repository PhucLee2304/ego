param(
    [Parameter(Mandatory = $true)][ValidateSet("reset-db", "seed")][string]$Action,
    [Parameter(Mandatory = $true)][string]$Service
)

$ErrorActionPreference = "Stop"
$databases = @{
    identity      = "ego-users-db"
    users         = "ego-users-db"
    topics        = "ego-topics-db"
    learning      = "ego-learning-db"
    notifications = "ego-notifications-db"
    vocabgram     = "vocabgram-db"
    dictionary    = "vocabgram-db"
    grammar       = "vocabgram-db"
}

if ($Action -eq "seed") {
    $allowed = @("topics", "learning", "grammar", "dictionary", "dictionary-topics", "vocabgram", "vocabgram-topics")
    if ($Service -notin $allowed) { throw "Seed is only available for: $($allowed -join ', ')" }
    docker compose --profile seed run --rm "$Service-seed"
    if ($LASTEXITCODE -ne 0) { throw "Seed failed for $Service" }
    exit
}

if (-not $databases.ContainsKey($Service)) { throw "Unsupported service: $Service" }
docker exec $databases[$Service] sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"'
if ($LASTEXITCODE -ne 0) { throw "Reset failed for $Service" }
$container = if ($Service -in @("users", "identity")) { "identity" } elseif ($Service -in @("dictionary", "grammar", "vocabgram")) { "vocabgram" } else { $Service }
docker compose restart $container