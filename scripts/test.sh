#!/usr/bin/env sh
set -eu

cleanup() {
  docker compose -p ego-test -f docker-compose.test.yml down -v --remove-orphans
}
trap cleanup EXIT INT TERM

docker compose -p ego-test -f docker-compose.test.yml up -d --wait
export EXAMS_TEST_DSN='host=localhost user=test password=test dbname=exams_test port=55435 sslmode=disable'
export CLASSROOMS_TEST_DSN='host=localhost user=test password=test dbname=classrooms_test port=55436 sslmode=disable'
go test ./...
go test -tags=integration ./services/exams/integration/... ./services/classrooms/integration/...
