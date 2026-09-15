package db

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

const UniqueViolationCode = "23505"

func IsUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	if pgErr.Code != UniqueViolationCode {
		return false
	}

	if constraint == "" {
		return true
	}

	return pgErr.ConstraintName == constraint
}
