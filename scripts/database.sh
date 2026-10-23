#!/usr/bin/env sh
set -eu

case "$ACTION:$SERVICE" in
  seed:topics|seed:exams|seed:learning)
    docker compose --profile seed run --rm "${SERVICE}-seed"
    ;;
  seed:*)
    echo "Seed is only available for topics, exams and learning" >&2
    exit 1
    ;;
  reset-db:identity|reset-db:users)
    docker exec users-db sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"'
    docker compose restart identity
    ;;
  reset-db:topics|reset-db:exams|reset-db:classrooms|reset-db:learning|reset-db:notifications)
    docker exec "${SERVICE}-db" sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"'
    docker compose restart "$SERVICE"
    ;;
  *)
    echo "Unsupported service: $SERVICE" >&2
    exit 1
    ;;
esac
