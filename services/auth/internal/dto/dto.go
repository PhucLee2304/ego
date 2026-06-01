package dto

type RefreshBody struct {
	RefreshToken string `json:"refreshToken" validate:"required"`
}

type RefreshResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

type LoginBody struct {
	IdToken string  `json:"idToken" validate:"required"`
	Name    *string `json:"name,omitempty" validate:"omitempty,min=1"`
}

type LoginResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}
