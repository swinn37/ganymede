package platform

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func withTwitchDeviceServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	previousDeviceURL, previousAuthURL := TwitchDeviceUrl, TwitchAuthUrl
	TwitchDeviceUrl, TwitchAuthUrl = server.URL+"/device", server.URL+"/token"
	t.Cleanup(func() {
		TwitchDeviceUrl, TwitchAuthUrl = previousDeviceURL, previousAuthURL
	})
}

func TestStartTwitchDeviceLogin(t *testing.T) {
	withTwitchDeviceServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("failed to parse form: %v", err)
		}
		if r.Method != http.MethodPost || r.URL.Path != "/device" || r.PostForm.Get("client_id") != twitchTVClientID {
			t.Errorf("unexpected device request: %s %s %v", r.Method, r.URL.Path, r.PostForm)
		}
		_, _ = w.Write([]byte(`{"device_code":"device","expires_in":1800,"interval":5,"user_code":"ABCDEFGH","verification_uri":"https://www.twitch.tv/activate"}`))
	})

	login, err := StartTwitchDeviceLogin(context.Background())
	if err != nil {
		t.Fatalf("StartTwitchDeviceLogin: %v", err)
	}
	if login.DeviceCode != "device" || login.UserCode != "ABCDEFGH" || login.ExpiresIn != 1800 || login.Interval != 5 {
		t.Errorf("unexpected login: %+v", login)
	}
	if want := "https://www.twitch.tv/activate?device-code=ABCDEFGH"; login.VerificationURI != want {
		t.Errorf("verification URI = %q, want %q", login.VerificationURI, want)
	}
}

func TestFinishTwitchDeviceLogin(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		wantToken string
		wantErr   error
	}{
		{name: "pending", status: http.StatusBadRequest, body: `{"status":400,"message":"authorization_pending"}`, wantErr: ErrTwitchDeviceLoginPending},
		{name: "slow down", status: http.StatusBadRequest, body: `{"status":400,"message":"slow_down"}`, wantErr: ErrTwitchDeviceLoginPending},
		{name: "expired", status: http.StatusBadRequest, body: `{"status":400,"message":"invalid device code"}`, wantErr: ErrTwitchDeviceLoginFailed},
		{name: "authorized", status: http.StatusOK, body: `{"access_token":"token","refresh_token":"refresh","scope":[],"token_type":"bearer"}`, wantToken: "token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withTwitchDeviceServer(t, func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseForm(); err != nil {
					t.Errorf("failed to parse form: %v", err)
				}
				if r.URL.Path != "/token" || r.PostForm.Get("client_id") != twitchTVClientID || r.PostForm.Get("device_code") != "device" ||
					r.PostForm.Get("grant_type") != "urn:ietf:params:oauth:grant-type:device_code" {
					t.Errorf("unexpected token request: %s %v", r.URL.Path, r.PostForm)
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			})

			token, err := FinishTwitchDeviceLogin(context.Background(), "device")
			if token != tt.wantToken || !errors.Is(err, tt.wantErr) {
				t.Errorf("got (%q, %v), want (%q, %v)", token, err, tt.wantToken, tt.wantErr)
			}
		})
	}

	t.Run("twitch unavailable", func(t *testing.T) {
		withTwitchDeviceServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		})

		_, err := FinishTwitchDeviceLogin(context.Background(), "device")
		if err == nil || errors.Is(err, ErrTwitchDeviceLoginPending) || errors.Is(err, ErrTwitchDeviceLoginFailed) {
			t.Errorf("got %v, want an upstream error", err)
		}
	})
}
