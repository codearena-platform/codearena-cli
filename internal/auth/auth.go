// Package auth implements the CodeArena IAM login flows used by the CLI and
// persists the resulting bearer token for other commands.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	// DefaultEndpoint points at the local development gateway. Production
	// installations should override it with --endpoint.
	DefaultEndpoint = "https://localhost:8080"
	// IAMBasePath is the gateway prefix that is reverse-proxied to IAM.
	IAMBasePath      = "/api/v1/iam"
	maxResponseBytes = 1 << 20
	requestTimeout   = 15 * time.Second
	callbackTimeout  = 5 * time.Minute
)

// Config is the on-disk representation of the CLI credentials. The identity is
// intentionally not stored here: the competition service derives it from the
// verified token on every request.
type Config struct {
	Token    string `json:"token"`
	ClientID string `json:"client_id,omitempty"`
}

// LoginOptions configures an IAM login attempt.
type LoginOptions struct {
	Endpoint      string
	ClientID      string
	ClientSecret  string
	RedirectURL   string
	AllowInsecure bool // development only: permit plaintext HTTP

	// HTTPClient and OpenBrowser are optional injection points for tests.
	HTTPClient  *http.Client
	OpenBrowser func(string) error
}

// Client talks to IAM through the gateway.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// ConfigPath gets the path to the CLI config file.
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolver diretório do usuário: %w", err)
	}
	dir := filepath.Join(home, ".codearena")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("criar diretório de configuração: %w", err)
	}
	return filepath.Join(dir, "config.json"), nil
}

// SaveConfig writes the auth configuration atomically with owner-only access.
func SaveConfig(cfg *Config) error {
	if cfg == nil || strings.TrimSpace(cfg.Token) == "" {
		return errors.New("token de acesso ausente")
	}
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("serializar configuração: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*")
	if err != nil {
		return fmt.Errorf("criar arquivo temporário: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return fmt.Errorf("proteger arquivo de configuração: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("escrever configuração: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("fechar configuração: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("salvar configuração: %w", err)
	}
	return nil
}

// ReadConfig returns the currently stored CLI configuration.
func ReadConfig() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("não autenticado. Execute 'codearena login'")
		}
		return nil, fmt.Errorf("ler configuração: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("formato de configuração inválido no arquivo %s: %w", path, err)
	}
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, fmt.Errorf("token de acesso ausente no arquivo %s. Execute 'codearena login'", path)
	}
	return &cfg, nil
}

// NewClient validates the endpoint and builds an HTTP client. HTTPS is the
// default; plaintext HTTP is only accepted when explicitly opted into.
func NewClient(endpoint string, allowInsecure bool) (*Client, error) {
	u, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return nil, fmt.Errorf("endpoint inválido: %w", err)
	}
	if u.Host == "" || u.User != nil {
		return nil, fmt.Errorf("endpoint inválido %q: informe scheme e host sem credenciais", endpoint)
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawQuery, u.Fragment = "", ""

	transport := http.DefaultTransport.(*http.Transport).Clone()
	switch u.Scheme {
	case "https":
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	case "http":
		if !allowInsecure {
			return nil, fmt.Errorf("endpoint HTTP %q não é permitido; use HTTPS ou --insecure apenas em desenvolvimento", endpoint)
		}
	default:
		return nil, fmt.Errorf("scheme não suportado %q em %q", u.Scheme, endpoint)
	}

	return &Client{
		BaseURL: strings.TrimRight(u.String(), "/"),
		HTTP:    &http.Client{Transport: transport, Timeout: requestTimeout},
	}, nil
}

// Login authenticates against IAM and saves the resulting token. Supplying a
// client secret uses the OAuth2 client-credentials flow (service accounts);
// otherwise the interactive PKCE authorization-code flow is used.
func Login(ctx context.Context, opts LoginOptions) (*Config, error) {
	client, err := NewClient(opts.Endpoint, opts.AllowInsecure)
	if err != nil {
		return nil, err
	}
	if opts.HTTPClient != nil {
		client.HTTP = opts.HTTPClient
	}
	if strings.TrimSpace(opts.ClientID) == "" {
		return nil, errors.New("--client-id é obrigatório")
	}

	var token string
	if strings.TrimSpace(opts.ClientSecret) != "" {
		token, err = client.ClientCredentials(ctx, opts.ClientID, opts.ClientSecret)
	} else {
		token, err = client.PKCE(ctx, opts.ClientID, opts.RedirectURL, opts.OpenBrowser)
	}
	if err != nil {
		return nil, err
	}

	cfg := &Config{Token: token, ClientID: opts.ClientID}
	if err := SaveConfig(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

type iamEnvelope struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type tokenData struct {
	AccessToken string `json:"access_token"`
}

type loginData struct {
	LoginURL string `json:"login_url"`
}

// ClientCredentials exchanges a service-account client id/secret for a token.
func (c *Client) ClientCredentials(ctx context.Context, clientID, clientSecret string) (string, error) {
	if strings.TrimSpace(clientID) == "" || strings.TrimSpace(clientSecret) == "" {
		return "", errors.New("client_id e client_secret são obrigatórios")
	}
	body, err := json.Marshal(map[string]string{
		"client_id":     clientID,
		"client_secret": clientSecret,
	})
	if err != nil {
		return "", fmt.Errorf("serializar credenciais: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url("/auth/client"), strings.NewReader(string(body)))
	if err != nil {
		return "", fmt.Errorf("criar requisição de credenciais: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	var data tokenData
	if err := c.do(req, &data); err != nil {
		return "", err
	}
	if data.AccessToken == "" {
		return "", errors.New("IAM não retornou access_token")
	}
	return data.AccessToken, nil
}

// PKCE performs the RFC 7636 authorization-code flow with a local callback
// listener. It returns the bearer token issued by IAM.
func (c *Client) PKCE(ctx context.Context, clientID, redirectURL string, openBrowser func(string) error) (string, error) {
	if strings.TrimSpace(clientID) == "" {
		return "", errors.New("client_id é obrigatório")
	}
	redirect, err := url.Parse(redirectURL)
	if err != nil || (redirect.Scheme != "http" && redirect.Scheme != "https") || redirect.Host == "" {
		return "", fmt.Errorf("redirect_url inválida %q", redirectURL)
	}
	if redirect.User != nil || redirect.RawQuery != "" || redirect.Fragment != "" {
		return "", fmt.Errorf("redirect_url não pode conter credenciais, query ou fragmento: %q", redirectURL)
	}

	verifier, err := randomToken(32)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	state, err := randomToken(24)
	if err != nil {
		return "", err
	}

	callbacks := make(chan callbackResult, 1)
	srv, err := startCallbackServer(redirect, state, callbacks)
	if err != nil {
		return "", err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	loginQuery := url.Values{
		"client_id":             {clientID},
		"state":                 {state},
		"redirect_url":          {redirectURL},
		"code_challenge_method": {"S256"},
		"code_challenge":        {challenge},
		"postback":              {"true"},
	}
	var login loginData
	if err := c.get(ctx, "/auth/login", loginQuery, &login); err != nil {
		return "", err
	}
	if strings.TrimSpace(login.LoginURL) == "" {
		return "", errors.New("IAM não retornou login_url")
	}

	if openBrowser == nil {
		openBrowser = openInBrowser
	}
	if err := openBrowser(login.LoginURL); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Não foi possível abrir o navegador: %v\n", err)
	}
	fmt.Fprintf(os.Stderr, "🌐 Se o navegador não abrir, acesse: %s\n", login.LoginURL)

	var result callbackResult
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("tempo esgotado aguardando o login: %w", ctx.Err())
	case result = <-callbacks:
		if result.err != "" {
			return "", fmt.Errorf("IAM retornou erro no callback: %s", result.err)
		}
	}

	verifyQuery := url.Values{
		"code":           {result.code},
		"client_id":      {clientID},
		"code_challenge": {verifier}, // VerifyToken expects the PKCE verifier.
		"state":          {state},
	}
	var data tokenData
	if err := c.get(ctx, "/auth/verify", verifyQuery, &data); err != nil {
		return "", err
	}
	if data.AccessToken == "" {
		return "", errors.New("IAM não retornou access_token")
	}
	return data.AccessToken, nil
}

// callbackResult carries the outcome of the browser redirect.
type callbackResult struct {
	code string
	err  string
}

func startCallbackServer(redirect *url.URL, expectedState string, results chan<- callbackResult) (*http.Server, error) {
	host := redirect.Host
	if !strings.Contains(host, ":") {
		port := "80"
		if redirect.Scheme == "https" {
			port = "443"
		}
		host = net.JoinHostPort(host, port)
	}
	listener, err := net.Listen("tcp", host)
	if err != nil {
		return nil, fmt.Errorf("iniciar callback local em %s: %w", host, err)
	}

	callbackPath := redirect.Path
	if callbackPath == "" {
		callbackPath = "/"
	}
	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		q := r.URL.Query()
		if got := q.Get("error"); got != "" {
			fmt.Fprint(w, "<h1>Falha no login</h1>")
			notify(results, callbackResult{err: got})
			return
		}
		if got := q.Get("state"); got != expectedState || got == "" {
			http.Error(w, "state inválido", http.StatusBadRequest)
			return
		}
		code := q.Get("code")
		if code == "" {
			http.Error(w, "código de autorização ausente", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, "<h1>Login concluído. Você pode fechar esta janela.</h1>")
		notify(results, callbackResult{code: code})
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			notify(results, callbackResult{err: err.Error()})
		}
	}()
	return srv, nil
}

func notify(results chan<- callbackResult, result callbackResult) {
	select {
	case results <- result:
	default:
	}
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	u := c.url(path)
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("criar requisição IAM: %w", err)
	}
	return c.do(req, out)
}

func (c *Client) url(path string) string {
	return c.BaseURL + IAMBasePath + path
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("requisição IAM falhou: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("ler resposta IAM: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("IAM respondeu com status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var env iamEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("resposta IAM inválida: %w", err)
	}
	if !env.Success {
		message := strings.TrimSpace(env.Message)
		if message == "" {
			message = "falha não especificada"
		}
		return fmt.Errorf("IAM rejeitou a operação: %s", message)
	}
	if len(env.Data) == 0 || string(env.Data) == "null" {
		return errors.New("resposta IAM não contém data")
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("decodificar data da resposta IAM: %w", err)
	}
	return nil
}

func randomToken(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("gerar valor aleatório: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func openInBrowser(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("não é seguro abrir URL com scheme %q", u.Scheme)
	}

	var name string
	var args []string
	switch {
	case os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != "":
		name, args = "xdg-open", []string{rawURL}
	case os.Getenv("CODEARENA_BROWSER") != "":
		name = os.Getenv("CODEARENA_BROWSER")
		args = []string{rawURL}
	default:
		return errors.New("nenhum navegador padrão detectado (defina CODEARENA_BROWSER)")
	}
	return exec.Command(name, args...).Start()
}
