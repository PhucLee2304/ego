package model

type AttemptDuration int

const (
	AttemptDuration10  AttemptDuration = 10
	AttemptDuration15  AttemptDuration = 15
	AttemptDuration20  AttemptDuration = 20
	AttemptDuration30  AttemptDuration = 30
	AttemptDuration45  AttemptDuration = 45
	AttemptDuration60  AttemptDuration = 60
	AttemptDuration75  AttemptDuration = 75
	AttemptDuration90  AttemptDuration = 90
	AttemptDuration120 AttemptDuration = 120
)

// GetTestDurationByExamType trả thời gian chuẩn (phút) của chế độ thi theo loại đề, giống
// thời gian thi thật: THPT 60 phút, TOEIC 120 phút. Loại khác trả false.
func GetTestDurationByExamType(examType ExamType) (int, bool) {
	switch examType {
	case ExamTypeTHPT:
		return int(AttemptDuration60), true
	case ExamTypeTOEIC:
		return int(AttemptDuration120), true
	default:
		return 0, false
	}
}
