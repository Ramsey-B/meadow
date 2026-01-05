package middleware

import (
	"strconv"
	"time"

	"github.com/Gobusters/ectologger"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func Logger(logger ectologger.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			req := c.Request()
			res := c.Response()
			start := time.Now()
			if err = next(c); err != nil {
				c.Error(err)
			}

			stop := time.Now()

			id := req.Header.Get(echo.HeaderXRequestID)
			if id == "" {
				id = res.Header().Get(echo.HeaderXRequestID)
				if id == "" {
					id = uuid.New().String()
				}
			}

			logger.WithContext(c.Request().Context()).WithFields(map[string]interface{}{
				"request_id":    id,
				"method":        req.Method,
				"uri":           req.RequestURI,
				"status":        res.Status,
				"referer":       req.Referer(),
				"route":         c.Path(),
				"remote_ip":     c.RealIP(),
				"protocol":      req.Proto,
				"host":          req.Host,
				"user_agent":    req.UserAgent(),
				"start_time":    start,
				"stop_time":     stop,
				"response_time": stop.Sub(start),
				"request_size":  req.Header.Get(echo.HeaderContentLength),
				"response_size": strconv.FormatInt(res.Size, 10),
			}).Info("Request")

			return nil
		}
	}
}

