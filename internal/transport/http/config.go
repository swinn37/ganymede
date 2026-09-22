package http

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"github.com/zibbp/ganymede/internal/config"
	"github.com/zibbp/ganymede/internal/platform"
)

// GetConfig godoc
//
//	@Summary		Get config
//	@Description	Get config
//	@Tags			config
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	config.Config
//	@Failure		500	{object}	utils.ErrorResponse
//	@Router			/config [get]
//	@Security		ApiKeyCookieAuth
//	@Security		ApiKeyAuth
func (h *Handler) GetConfig(c echo.Context) error {
	config := config.Get()
	return SuccessResponse(c, config, "config")
}

// UpdateConfig godoc
//
//	@Summary		Update config
//	@Description	Update config
//	@Tags			config
//	@Accept			json
//	@Produce		json
//	@Param			body	body		config.Config	true	"Config"
//	@Success		200		{object}	config.Config
//	@Failure		400		{object}	utils.ErrorResponse
//	@Failure		500		{object}	utils.ErrorResponse
//	@Router			/config [put]
//	@Security		ApiKeyCookieAuth
//	@Security		ApiKeyAuth
func (h *Handler) UpdateConfig(c echo.Context) error {
	conf := new(config.Config)
	if err := c.Bind(conf); err != nil {
		return ErrorResponse(c, http.StatusBadRequest, err.Error())
	}

	if err := c.Validate(conf); err != nil {
		return ErrorResponse(c, http.StatusBadRequest, err.Error())
	}

	err := config.UpdateConfig(conf)
	if err != nil {
		return ErrorResponse(c, http.StatusInternalServerError, err.Error())
	}

	return SuccessResponse(c, conf, "config updated")
}

type PollTwitchLoginRequest struct {
	DeviceCode string `json:"device_code" validate:"required,max=255"`
}

type PollTwitchLoginResponse struct {
	Status      string `json:"status" enums:"pending,authorized"`
	TwitchToken string `json:"twitch_token,omitempty"` // Set once authorized; already saved to the config.
}

// StartTwitchLogin godoc
//
//	@Summary		Start Twitch login
//	@Description	Request a code to authorize at the returned Twitch URL; polling it then saves the account's Twitch token.
//	@Tags			config
//	@Produce		json
//	@Success		200	{object}	platform.TwitchDeviceLogin
//	@Failure		502	{object}	utils.ErrorResponse
//	@Router			/config/twitch-login [post]
//	@Security		ApiKeyCookieAuth
//	@Security		ApiKeyAuth
func (h *Handler) StartTwitchLogin(c echo.Context) error {
	login, err := platform.StartTwitchDeviceLogin(c.Request().Context())
	if err != nil {
		return ErrorResponse(c, http.StatusBadGateway, err.Error())
	}
	return SuccessResponse(c, login, "twitch login started")
}

// PollTwitchLogin godoc
//
//	@Summary		Poll Twitch login
//	@Description	Check whether the Twitch login code was authorized. Once it is, the account's token is saved as the Twitch token.
//	@Tags			config
//	@Accept			json
//	@Produce		json
//	@Param			body	body		PollTwitchLoginRequest	true	"Device code from the start of the login"
//	@Success		200		{object}	PollTwitchLoginResponse
//	@Failure		400		{object}	utils.ErrorResponse
//	@Failure		500		{object}	utils.ErrorResponse
//	@Failure		502		{object}	utils.ErrorResponse
//	@Router			/config/twitch-login/poll [post]
//	@Security		ApiKeyCookieAuth
//	@Security		ApiKeyAuth
func (h *Handler) PollTwitchLogin(c echo.Context) error {
	req := new(PollTwitchLoginRequest)
	if err := c.Bind(req); err != nil {
		return ErrorResponse(c, http.StatusBadRequest, err.Error())
	}
	if err := c.Validate(req); err != nil {
		return ErrorResponse(c, http.StatusBadRequest, err.Error())
	}

	// Twitch failures never map to 401: the frontend treats a 401 as an expired Ganymede session.
	token, err := platform.FinishTwitchDeviceLogin(c.Request().Context(), req.DeviceCode)
	switch {
	case errors.Is(err, platform.ErrTwitchDeviceLoginPending):
		return SuccessResponse(c, PollTwitchLoginResponse{Status: "pending"}, err.Error())
	case errors.Is(err, platform.ErrTwitchDeviceLoginFailed):
		return ErrorResponse(c, http.StatusBadRequest, err.Error())
	case err != nil:
		return ErrorResponse(c, http.StatusBadGateway, err.Error())
	}

	conf := config.Get()
	conf.Parameters.TwitchToken = token
	if err := config.UpdateConfig(conf); err != nil {
		return ErrorResponse(c, http.StatusInternalServerError, err.Error())
	}
	log.Info().Msg("twitch token saved from twitch login")

	return SuccessResponse(c, PollTwitchLoginResponse{Status: "authorized", TwitchToken: token}, "twitch token saved")
}
