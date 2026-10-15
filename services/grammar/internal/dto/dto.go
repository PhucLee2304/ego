package dto

// ---- Practice DTOs ----

type PracticeResponse struct {
	ID          uint     `json:"id"`
	Question    string   `json:"question"`
	Options     []string `json:"options"`
	Correct     string   `json:"correct"`
	Explanation string   `json:"explanation"`
	SortOrder   int      `json:"sortOrder"`
}

// ---- Unit DTOs ----

type UnitSummaryResponse struct {
	ID        uint   `json:"id"`
	Unit      int    `json:"unit"`
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
	Total     int    `json:"total"`
}

type UnitDetailResponse struct {
	ID        uint               `json:"id"`
	Unit      int                `json:"unit"`
	Title     string             `json:"title"`
	Theory    string             `json:"theory"`
	Examples  []string           `json:"examples"`
	Practices []PracticeResponse `json:"practices"`
	Completed bool               `json:"completed"`
}

// ---- Curriculum (List all units) ----

type CurriculumResponse struct {
	Data  []UnitSummaryResponse `json:"data"`
	Total int                   `json:"total"`
}

// ---- Progress DTOs ----

type SubmitProgressRequest struct {
	UnitID    uint `json:"unitId" validate:"required"`
	Completed bool `json:"completed"`
}

type ProgressResponse struct {
	ID        uint   `json:"id"`
	UserID    string `json:"userId"`
	UnitID    uint   `json:"unitId"`
	Completed bool   `json:"completed"`
	UpdatedAt string `json:"updatedAt"`
}
