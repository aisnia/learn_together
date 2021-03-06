package casbinmiddleware

import (
	"context"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"learn_together/api/authcontrol"
	"net/http"
)

type (
	// Config defines the config for CasbinAuth middleware.
	Config struct {
		// Skipper defines a function to skip middleware.
		Skipper middleware.Skipper

		// Enforcer CasbinAuth main rule.
		// Required.
		Enforcer *authcontrol.Enforcer

		// Method to get the username - defaults to using basic auth
		UserGetter func(c echo.Context) (string, error)
	}
)

var (
	// DefaultConfig is the default CasbinAuth middleware config.
	DefaultConfig = Config{
		Skipper: middleware.DefaultSkipper,
		UserGetter: func(c echo.Context) (string, error) {
			username, _, _ := c.Request().BasicAuth()
			return username, nil
		},
	}
)

// Middleware returns a CasbinAuth middleware.
//
// For valid credentials it calls the next handler.
// For missing or invalid credentials, it sends "401 - Unauthorized" response.
func Middleware(ce *authcontrol.Enforcer) echo.MiddlewareFunc {
	c := DefaultConfig
	c.Enforcer = ce
	return MiddlewareWithConfig(c)
}

// MiddlewareWithConfig returns a CasbinAuth middleware with config.
// See `Middleware()`.
func MiddlewareWithConfig(config Config) echo.MiddlewareFunc {
	// Defaults
	if config.Skipper == nil {
		config.Skipper = DefaultConfig.Skipper
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Skipper(c) {
				return next(c)
			}

			if pass, err := config.CheckPermission(c); err == nil && pass {
				return next(c)
			} else if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
			}

			return echo.ErrForbidden
		}
	}
}

// GetUserName gets the user name from the request.
// It calls the UserGetter field of the Config struct that allows the caller to customize user identification.
func (a *Config) GetUserName(c echo.Context) (string, error) {
	username, err := a.UserGetter(c)
	return username, err
}

// CheckPermission checks the user/method/path combination from the request.
// Returns true (permission granted) or false (permission forbidden)
func (a *Config) CheckPermission(c echo.Context) (bool, error) {
	user, err := a.GetUserName(c)
	if err != nil {
		// Fail safe and do not propagate
		return false, nil
	}
	method := c.Request().Method
	path := c.Request().URL.Path
	return a.Enforcer.Enforce(context.Background(), user, path, method)
}
