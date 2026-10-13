$ErrorActionPreference = "Stop"
$compose = @("compose", "-p", "ego-test", "-f", "docker-compose.test.yml")

try {
    docker @compose up -d --wait
    if ($LASTEXITCODE -ne 0) { throw "Could not start integration databases" }

    $env:EXAMS_TEST_DSN = "host=localhost user=test password=test dbname=exams_test port=55435 sslmode=disable"
    $env:CLASSROOMS_TEST_DSN = "host=localhost user=test password=test dbname=classrooms_test port=55436 sslmode=disable"
    $env:NOTIFICATIONS_TEST_DSN = "host=localhost user=test password=test dbname=notifications_test port=55437 sslmode=disable"
    $env:POSTS_TEST_DSN = "host=localhost user=test password=test dbname=posts_test port=55438 sslmode=disable"

    go test ./...
    if ($LASTEXITCODE -ne 0) { throw "Unit tests failed" }
    go test -tags=integration ./services/exams/integration/... ./services/classrooms/integration/... ./services/notifications/integration/... ./services/posts/integration/...
    if ($LASTEXITCODE -ne 0) { throw "Integration tests failed" }
} finally {
    docker @compose down -v --remove-orphans
}
