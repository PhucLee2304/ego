package model

const AttemptActiveUserUniqueIndex = "idx_attempts_active_user_unique"
const AttemptClassroomAssignmentUniqueIndex = "idx_attempts_classroom_assignment_unique"
const OptionCorrectQuestionUniqueIndex = "idx_options_correct_question_unique"
const OptionQuestionKeyUniqueIndex = "idx_options_question_key_unique"

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

const CreateOptionCorrectQuestionUniqueIndexSQL = `
	CREATE UNIQUE INDEX IF NOT EXISTS idx_options_correct_question_unique
	ON options (question_id)
	WHERE is_correct = true AND deleted_at IS NULL;
`

const CreateOptionQuestionKeyUniqueIndexSQL = `
	CREATE UNIQUE INDEX IF NOT EXISTS idx_options_question_key_unique
	ON options (question_id, key)
	WHERE deleted_at IS NULL;
`

const CreateAssignmentQuestionOptionsConstraintSQL = `
	CREATE OR REPLACE FUNCTION validate_assignment_question_options(question_id_to_check bigint)
	RETURNS void AS $$
	DECLARE
		exam_type text;
		option_count integer;
		correct_count integer;
		valid_key_count integer;
	BEGIN
		SELECT exams.type
		INTO exam_type
		FROM questions
		JOIN sections ON sections.id = questions.section_id AND sections.deleted_at IS NULL
		JOIN exams ON exams.id = sections.exam_id AND exams.deleted_at IS NULL
		WHERE questions.id = question_id_to_check AND questions.deleted_at IS NULL;

		IF exam_type IS DISTINCT FROM 'ASSIGNMENT' THEN
			RETURN;
		END IF;

		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE is_correct = true),
			COUNT(*) FILTER (WHERE key IN ('A', 'B', 'C', 'D'))
		INTO option_count, correct_count, valid_key_count
		FROM options
		WHERE question_id = question_id_to_check AND deleted_at IS NULL;

		IF option_count <> 4 OR valid_key_count <> 4 OR correct_count <> 1 THEN
			RAISE EXCEPTION 'Assignment question % must have exactly options A, B, C, D and one correct answer', question_id_to_check
				USING ERRCODE = '23514';
		END IF;
	END;
	$$ LANGUAGE plpgsql;

	CREATE OR REPLACE FUNCTION validate_assignment_question_options_from_option()
	RETURNS trigger AS $$
	BEGIN
		IF TG_OP = 'DELETE' THEN
			PERFORM validate_assignment_question_options(OLD.question_id);
		ELSIF TG_OP = 'UPDATE' THEN
			IF OLD.question_id IS DISTINCT FROM NEW.question_id THEN
				PERFORM validate_assignment_question_options(OLD.question_id);
			END IF;
			PERFORM validate_assignment_question_options(NEW.question_id);
		ELSE
			PERFORM validate_assignment_question_options(NEW.question_id);
		END IF;
		RETURN COALESCE(NEW, OLD);
	END;
	$$ LANGUAGE plpgsql;

	CREATE OR REPLACE FUNCTION validate_assignment_question_options_from_question()
	RETURNS trigger AS $$
	BEGIN
		PERFORM validate_assignment_question_options(NEW.id);
		RETURN NEW;
	END;
	$$ LANGUAGE plpgsql;

	DROP TRIGGER IF EXISTS validate_assignment_question_options_on_options ON options;
	CREATE CONSTRAINT TRIGGER validate_assignment_question_options_on_options
	AFTER INSERT OR UPDATE OR DELETE ON options
	DEFERRABLE INITIALLY DEFERRED
	FOR EACH ROW
	EXECUTE FUNCTION validate_assignment_question_options_from_option();

	DROP TRIGGER IF EXISTS validate_assignment_question_options_on_questions ON questions;
	CREATE CONSTRAINT TRIGGER validate_assignment_question_options_on_questions
	AFTER INSERT OR UPDATE ON questions
	DEFERRABLE INITIALLY DEFERRED
	FOR EACH ROW
	EXECUTE FUNCTION validate_assignment_question_options_from_question();
`

const CreateRoomParticipantOpenSessionUniqueIndexSQL = `
	CREATE UNIQUE INDEX IF NOT EXISTS idx_room_participants_open_session
	ON room_participants (room_id, user_id)
	WHERE left_at IS NULL AND deleted_at IS NULL;
`
