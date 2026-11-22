package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewClientRequiresTLSUnlessOptedIn(t *testing.T) {
	if _, err := NewClient("http://localhost:8080", false); err == nil || !strings.Contains(err.Error(), "--insecure") {
		t.Fatalf("plaintext endpoint without opt-in: got %v, want --insecure error", err)
	}
	client, err := NewClient("http://localhost:8080", true)
	if err != nil || client.BaseURL != "http://localhost:8080" {
		t.Fatalf("plaintext endpoint with opt-in: client=%+v err=%v", client, err)
	}
	client, err = NewClient("https://localhost:8080", false)
	if err != nil || client.BaseURL != "https://localhost:8080" {
		t.Fatalf("https endpoint: client=%+v err=%v", client, err)
	}
	if _, err := NewClient("ftp://localhost:8080", true); err == nil {
		t.Fatal("unexpected success for ftp endpoint")
	}
	if _, err := NewClient("https://user:pass@localhost:8080", false); err == nil {
		t.Fatal("unexpected success for endpoint with credentials")
	}
}

func TestClientCredentialsParsesIAMEnvelope(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody map[string]string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request: %v", err)
		}
		fmt.Fprint(w, `{"success":true,"message":"Authentication successful","data":{"access_token":"jwt-from-iam"}}`)
	}))
	defer srv.Close()

	client := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	token, err := client.ClientCredentials(context.Background(), "cli-client", "s3cret")
	if err != nil {
		t.Fatalf("ClientCredentials: %v", err)
	}
	if token != "jwt-from-iam" {
		t.Fatalf("token = %q, want jwt-from-iam", token)
	}
	if gotMethod != http.MethodPost || gotPath != IAMBasePath+"/auth/client" {
		t.Fatalf("request = %s %s, want POST %s/auth/client", gotMethod, gotPath, IAMBasePath)
	}
	if gotBody["client_id"] != "cli-client" || gotBody["client_secret"] != "s3cret" {
		t.Fatalf("request body = %v", gotBody)
	}
}

func TestClientCredentialsSurfacesIAMFailures(t *testing.T) {
	tests := []struct {
		name string
		code int
		body string
		want string
	}{
		{"envelope false", http.StatusUnauthorized, `{"success":false,"message":"bad secret"}`, "bad secret"},
		{"missing data", http.StatusOK, `{"success":true}`, "data"},
		{"missing token", http.StatusOK, `{"success":true,"data":{}}`, "access_token"},
		{"http error", http.StatusInternalServerError, `boom`, "500"},
		{"garbage", http.StatusOK, `{`, "inválida"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.code)
				fmt.Fprint(w, tc.body)
			}))
			defer srv.Close()
			client := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
			if _, err := client.ClientCredentials(context.Background(), "id", "secret"); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestLoginClientCredentialsSavesToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"success":true,"data":{"access_token":"service-token"}}`)
	}))
	defer srv.Close()

	cfg, err := Login(context.Background(), LoginOptions{
		Endpoint:     srv.URL,
		ClientID:     "cli-client",
		ClientSecret: "s3cret",
		HTTPClient:   srv.Client(),
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if cfg.Token != "service-token" {
		t.Fatalf("token = %q", cfg.Token)
	}

	path, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("config mode = %o, want 600", info.Mode().Perm())
	}
	stored, err := ReadConfig()
	if err != nil || stored.Token != "service-token" || stored.ClientID != "cli-client" {
		t.Fatalf("ReadConfig = %+v, %v", stored, err)
	}
}

func TestPKCEFlow(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var mu sync.Mutex
	var challenge, state, redirectURL string
	var verifyQuery url.Values
	var srv *httptest.Server

	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case IAMBasePath + "/auth/login":
			if r.URL.Query().Get("code_challenge_method") != "S256" || r.URL.Query().Get("postback") != "true" {
				t.Errorf("unexpected login query: %v", r.URL.Query())
			}
			mu.Lock()
			challenge = r.URL.Query().Get("code_challenge")
			state = r.URL.Query().Get("state")
			redirectURL = r.URL.Query().Get("redirect_url")
			mu.Unlock()
			loginURL := srv.URL + "/authorize?" + url.Values{"state": {state}}.Encode()
			json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"message": "Login URL generated successfully",
				"data":    map[string]string{"login_url": loginURL},
			})
		case "/authorize":
			mu.Lock()
			callback, callbackState := redirectURL, state
			mu.Unlock()
			target, err := url.Parse(callback)
			if err != nil {
				t.Errorf("parse redirect: %v", err)
				return
			}
			q := target.Query()
			q.Set("code", "auth-code-123")
			q.Set("state", callbackState)
			target.RawQuery = q.Encode()
			http.Redirect(w, r, target.String(), http.StatusFound)
		case IAMBasePath + "/auth/verify":
			mu.Lock()
			verifyQuery = r.URL.Query()
			expectedChallenge := challenge
			mu.Unlock()
			if verifyQuery.Get("code") != "auth-code-123" || verifyQuery.Get("client_id") != "public-client" {
				t.Errorf("unexpected verify query: %v", verifyQuery)
			}
			if got := verifyQuery.Get("code_challenge"); got == "" {
				t.Error("missing PKCE verifier")
			} else {
				sum := sha256.Sum256([]byte(got))
				if base64.RawURLEncoding.EncodeToString(sum[:]) != expectedChallenge {
					t.Errorf("PKCE verifier does not match challenge")
				}
			}
			fmt.Fprint(w, `{"success":true,"data":{"access_token":"user-token"}}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cfg, err := Login(ctx, LoginOptions{
		Endpoint:    srv.URL,
		ClientID:    "public-client",
		RedirectURL: "http://127.0.0.1:18484/callback",
		HTTPClient:  srv.Client(),
		OpenBrowser: func(loginURL string) error {
			resp, err := srv.Client().Get(loginURL)
			if err != nil {
				return err
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			return resp.Body.Close()
		},
	})
	if err != nil {
		t.Fatalf("PKCE login: %v", err)
	}
	if cfg.Token != "user-token" {
		t.Fatalf("token = %q, want user-token", cfg.Token)
	}
	if verifyQuery.Get("state") != state {
		t.Fatalf("verify state = %q, want %q", verifyQuery.Get("state"), state)
	}
}

func TestPKCEIgnoresCallbackWithWrongState(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/auth/login") {
			fmt.Fprint(w, `{"success":true,"data":{"login_url":"`+srv.URL+`/authorize"}}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_, err := Login(ctx, LoginOptions{
		Endpoint:    srv.URL,
		ClientID:    "public-client",
		RedirectURL: "http://127.0.0.1:18485/callback",
		HTTPClient:  srv.Client(),
		OpenBrowser: func(loginURL string) error {
			resp, err := srv.Client().Get("http://127.0.0.1:18485/callback?code=x&state=wrong")
			if err != nil {
				return err
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			return resp.Body.Close()
		},
	})
	if err == nil || !strings.Contains(err.Error(), "tempo esgotado") {
		t.Fatalf("error = %v, want timeout because state mismatched", err)
	}
}

func TestConfigHelpers(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, err := ReadConfig(); err == nil || !strings.Contains(err.Error(), "não autenticado") {
		t.Fatalf("missing config error = %v", err)
	}
	if err := SaveConfig(&Config{}); err == nil {
		t.Fatal("empty token accepted")
	}
	if err := SaveConfig(&Config{Token: "abc"}); err != nil {
		t.Fatal(err)
	}
	path, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "config.json" {
		t.Fatalf("unexpected config path %s", path)
	}
}
