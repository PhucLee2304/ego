package validatorx

import (
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

func New() *validator.Validate {
	v := validator.New()
	_ = v.RegisterValidation("notblank", func(fl validator.FieldLevel) bool {
		field := fl.Field()
		for field.Kind() == reflect.Pointer {
			if field.IsNil() {
				return true
			}
			field = field.Elem()
		}

		if field.Kind() != reflect.String {
			return false
		}

		return strings.TrimSpace(field.String()) != ""
	})
	return v
}
