package config

import (
	"errors"

	"github.com/go-playground/validator/v10"
)

func validate(config any) error {
	v := validator.New()
	if err := v.Struct(config); err != nil {
		var errs []error
		for _, e := range err.(validator.ValidationErrors) {
			errs = append(errs, e.(error))
		}

		return errors.Join(errs...)
	}

	return nil
}
