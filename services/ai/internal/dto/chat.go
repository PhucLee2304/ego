package dto

// ChatRequest là body của POST /chat
type ChatRequest struct {
	Message string `json:"message" validate:"required,min=1,max=2000"`
}

// ChatResponse là response của POST /chat
type ChatResponse struct {
	Answer string `json:"answer"`
}
