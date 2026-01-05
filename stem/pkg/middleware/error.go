package middleware

import (
	"net/http"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/stem/pkg/context"
	"github.com/Ramsey-B/stem/pkg/tracing"
	"github.com/labstack/echo/v4"
)

type ErrorResponse struct {
	Message   string         `json:"message"`
	RequestID string         `json:"request_id"`
	TraceID   string         `json:"trace_id"`
	Meta      map[string]any `json:"meta"`
}

func Error(logger ectologger.Logger) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		ctx := c.Request().Context()
		logger.WithContext(ctx).WithError(err).Error("api is returning an error")
		// Check if the response is already committed
		if c.Response().Committed {
			return
		}

		// Default response
		code := http.StatusInternalServerError
		message := "Internal Server Error"
		meta := map[string]any{}

		// Handle specific Echo errors
		if he, ok := err.(*echo.HTTPError); ok {
			code = he.Code
			if msg, ok := he.Message.(string); ok {
				message = msg
			}
		}

		if ok := httperror.IsHTTPError(err); ok {
			httperr := httperror.ToHTTPError(err)
			code = httperror.GetStatusCode(err)
			message = httperr.Error()
			meta = httperr.Meta
		}
		requestID := context.GetRequestID(ctx)
		traceID := tracing.GetTraceID(ctx)

		// Return a JSON response
		_ = c.JSON(code, ErrorResponse{
			Message:   message,
			RequestID: requestID,
			TraceID:   traceID,
			Meta:      meta,
		})
	}
}

