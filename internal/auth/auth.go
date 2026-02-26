package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

type Config struct {
	Token string `json:"token"`
	Email string `json:"email"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken string `json:"access_token"`
	// Outros campos omitidos por simplicidade
}

// ConfigPath gets the path to the CLI config file
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".codearena")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Login authenticates the user with the given credentials and saves the token
func Login(gatewayURL, email, password string) error {
	reqBody, _ := json.Marshal(loginRequest{
		Email:    email,
		Password: password,
	})

	url := fmt.Sprintf("%s/api/v1/auth/login", gatewayURL)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("erro de rede ao logar: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("falha no login (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var lResp loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&lResp); err != nil {
		return fmt.Errorf("erro ao ler token digital: %w", err)
	}

	cfg := Config{
		Token: lResp.AccessToken,
		Email: email,
	}

	return SaveConfig(&cfg)
}

// SaveConfig writes the auth configurations to the disk
func SaveConfig(cfg *Config) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// ReadConfig returns the currently stored CLI configuration
func ReadConfig() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("não autenticado. Execute 'codearena login'")
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("formato de configuração inválido no arquivo %s: %w", path, err)
	}

	return &cfg, nil
}
