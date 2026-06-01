package dto

type User struct {
	ID     uint    `json:"id"`
	Email  string  `json:"email"`
	Name   string  `json:"name"`
	Avatar *string `json:"avatar"`
}

type UpdateUserBody struct {
	Name   *string `json:"name,omitempty" validate:"omitempty,min=1"`
	Avatar *string `json:"avatar,omitempty" validate:"omitempty,min=1"`
}
