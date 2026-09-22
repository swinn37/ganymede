package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TwitchDeviceUrl starts a device login; the device code is then exchanged at TwitchAuthUrl.
var TwitchDeviceUrl = "https://id.twitch.tv/oauth2/device"

var (
	// ErrTwitchDeviceLoginPending means the user has not authorized the device code yet.
	ErrTwitchDeviceLoginPending = errors.New("waiting for twitch authorization")
	// ErrTwitchDeviceLoginFailed means the device code expired or the user denied access.
	ErrTwitchDeviceLoginFailed = errors.New("twitch login failed")
)

var twitchDeviceHTTPClient = &http.Client{Timeout: 15 * time.Second}

// TwitchDeviceLogin is a code the user enters at its verification URI to authorize Ganymede.
type TwitchDeviceLogin struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"` // Seconds before the code expires.
	Interval        int    `json:"interval"`   // Seconds to wait between polls.
}

// StartTwitchDeviceLogin requests a device code for Twitch's TV app, whose tokens unlock
// subscriber-only and ad-free playback like the website's auth-token cookie.
func StartTwitchDeviceLogin(ctx context.Context) (*TwitchDeviceLogin, error) {
	body, status, err := twitchPostForm(ctx, TwitchDeviceUrl, url.Values{
		"client_id": {twitchTVClientID},
		"scopes":    {""},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start twitch login: %w", err)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("failed to start twitch login: %s", twitchOAuthMessage(body, status))
	}

	var login TwitchDeviceLogin
	if err := json.Unmarshal(body, &login); err != nil {
		return nil, fmt.Errorf("failed to unmarshal twitch device code: %w", err)
	}
	if login.DeviceCode == "" || login.VerificationURI == "" {
		return nil, fmt.Errorf("twitch returned an incomplete device code")
	}
	login.VerificationURI = prefillTwitchDeviceCode(login.VerificationURI, login.UserCode)

	return &login, nil
}

// FinishTwitchDeviceLogin exchanges an authorized device code for the user's access token.
// It returns ErrTwitchDeviceLoginPending until the user authorizes the code.
func FinishTwitchDeviceLogin(ctx context.Context, deviceCode string) (string, error) {
	body, status, err := twitchPostForm(ctx, TwitchAuthUrl, url.Values{
		"client_id":   {twitchTVClientID},
		"scopes":      {""},
		"device_code": {deviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	})
	if err != nil {
		return "", fmt.Errorf("failed to get twitch token: %w", err)
	}

	switch {
	case status == http.StatusOK:
		var token struct {
			AccessToken string `json:"access_token"`
		}
		if err := json.Unmarshal(body, &token); err != nil {
			return "", fmt.Errorf("failed to unmarshal twitch token: %w", err)
		}
		if token.AccessToken == "" {
			return "", fmt.Errorf("twitch returned an empty token")
		}
		return token.AccessToken, nil
	case status >= http.StatusInternalServerError:
		return "", fmt.Errorf("failed to get twitch token: %s", twitchOAuthMessage(body, status))
	}

	message := twitchOAuthMessage(body, status)
	if message == "authorization_pending" || message == "slow_down" {
		return "", ErrTwitchDeviceLoginPending
	}
	return "", fmt.Errorf("%w: %s", ErrTwitchDeviceLoginFailed, message)
}

func twitchPostForm(ctx context.Context, endpoint string, form url.Values) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := twitchDeviceHTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}

// twitchOAuthMessage returns the message of a Twitch OAuth error, such as "authorization_pending".
func twitchOAuthMessage(body []byte, status int) string {
	var resp struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &resp) == nil && resp.Message != "" {
		return resp.Message
	}
	return fmt.Sprintf("status %d", status)
}

// prefillTwitchDeviceCode adds the user code to the activation link so the user only confirms it.
func prefillTwitchDeviceCode(verificationURI, userCode string) string {
	u, err := url.Parse(verificationURI)
	if err != nil || userCode == "" {
		return verificationURI
	}
	q := u.Query()
	if q.Get("device-code") == "" {
		q.Set("device-code", userCode)
		u.RawQuery = q.Encode()
	}
	return u.String()
}
