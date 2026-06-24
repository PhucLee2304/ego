package dto

type User struct {
	ID     uint    `json:"id"`
	Email  string  `json:"email"`
	Name   string  `json:"name"`
	Avatar *string `json:"avatar"`
	Role   string  `json:"role"`
}

type UpdateUserBody struct {
	Name   *string `json:"name,omitempty" validate:"omitempty,notblank,max=100"`
	Avatar *string `json:"avatar,omitempty" validate:"omitempty,notblank,url"`
}
