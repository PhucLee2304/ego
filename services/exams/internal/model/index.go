package model

const AttemptActiveUserUniqueIndex = "idx_attempts_active_user_unique"
const AttemptClassroomAssignmentUniqueIndex = "idx_attempts_classroom_assignment_unique"
const OutboxEventTypeAggregateUniqueIndex = "idx_outbox_events_type_aggregate_unique"

const CreateAttemptActiveUserUniqueIndexSQL = `
	CREATE UNIQUE INDEX IF NOT EXISTS idx_attempts_active_user_unique
	ON attempts (user_id)
	WHERE status = 'ACTIVE' AND deleted_at IS NULL;
`

const CreateAttemptClassroomAssignmentUniqueIndexSQL = `
	CREATE UNIQUE INDEX IF NOT EXISTS idx_attempts_classroom_assignment_unique
	ON attempts (context_id, user_id)
	WHERE context_type = 'CLASSROOM_ASSIGNMENT' AND context_id IS NOT NULL AND deleted_at IS NULL;
`

const CreateOutboxEventTypeAggregateUniqueIndexSQL = `
	CREATE UNIQUE INDEX IF NOT EXISTS idx_outbox_events_type_aggregate_unique
	ON outbox_events (type, aggregate_id)
	WHERE aggregate_id IS NOT NULL AND deleted_at IS NULL;
`
