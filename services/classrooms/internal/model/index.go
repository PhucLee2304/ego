package model

const CreateRoomParticipantOpenSessionUniqueIndexSQL = `
	CREATE UNIQUE INDEX IF NOT EXISTS idx_room_participants_open_session
	ON room_participants (room_id, user_id)
	WHERE left_at IS NULL AND deleted_at IS NULL;
`
