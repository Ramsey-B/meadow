package utils

import (
	"net/http"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/labstack/echo/v4"
)

func BindRequest[T any](c echo.Context) (T, error) {
	var v T

	if err := c.Bind(&v); err != nil {
		return v, httperror.WrapError(http.StatusBadRequest, err)
	}

	if v, err := Validate(v); err != nil {
		return v, httperror.WrapError(http.StatusBadRequest, err)
	}

	return v, nil
}
