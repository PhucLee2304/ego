#!/usr/bin/env sh
set -eu

cleanup() {
  docker compose -p ego-test -f docker-compose.test.yml down -v --remove-orphans
}
trap cleanup EXIT INT TERM

docker compose -p ego-test -f docker-compose.test.yml up -d --wait
export LEARNING_TEST_DSN='host=localhost user=test password=test dbname=learning_test port=55440 sslmode=disable'
export NOTIFICATIONS_TEST_DSN='host=localhost user=test password=test dbname=notifications_test port=55437 sslmode=disable'
export POSTS_TEST_DSN='host=localhost user=test password=test dbname=posts_test port=55438 sslmode=disable'
export IDENTITY_TEST_DSN='host=localhost user=test password=test dbname=identity_test port=55439 sslmode=disable'
export LIVEKIT_TEST_URL='ws://localhost:17880'
export LIVEKIT_TEST_API_KEY='devkey'
export LIVEKIT_TEST_API_SECRET='local-development-secret-key-000001'
# Không dùng ./...: data/<svc>-db thuộc user postgres, go không đọc được thư mục đó.
go test ./api/... ./platform/... ./services/...
go test -tags=integration ./services/learning/integration/... ./services/notifications/integration/... ./services/posts/integration/... ./services/identity/integration/...
