package model

const AttemptActiveUserUniqueIndex = "idx_attempts_active_user_unique"

const CreateAttemptActiveUserUniqueIndexSQL = `
	CREATE UNIQUE INDEX IF NOT EXISTS idx_attempts_active_user_unique
	ON attempts (user_id)
	WHERE status = 'ACTIVE' AND deleted_at IS NULL;
`
