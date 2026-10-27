package dto

type RefreshBody struct {
	RefreshToken string `json:"refreshToken" validate:"required,notblank"`
}

type RefreshResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

type LoginBody struct {
	IdToken string  `json:"idToken" validate:"required,notblank"`
	Name    *string `json:"name,omitempty" validate:"omitempty,notblank"`
}

type LoginResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}
