package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/codearena-platform/codearena-cli/internal/auth"
	"github.com/spf13/cobra"
)

func newLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Autentica na CodeArena via IAM",
		Long: `Autentica na CodeArena usando os fluxos OAuth suportados pelo IAM.

Com --client-id e --client-secret usa o fluxo client-credentials (contas de
serviço). Com apenas --client-id inicia o fluxo PKCE (authorization code),
abrindo o navegador para o login interativo.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			endpoint, err := cmd.Flags().GetString("endpoint")
			if err != nil {
				return err
			}
			clientID, err := cmd.Flags().GetString("client-id")
			if err != nil {
				return err
			}
			clientSecret, err := cmd.Flags().GetString("client-secret")
			if err != nil {
				return err
			}
			redirectURL, err := cmd.Flags().GetString("redirect-url")
			if err != nil {
				return err
			}
			allowInsecure, err := cmd.Flags().GetBool("insecure")
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "🔐 Autenticando em %s...\n", endpoint)

			ctx, cancel := context.WithTimeout(cmd.Context(), 6*time.Minute)
			defer cancel()

			cfg, err := auth.Login(ctx, auth.LoginOptions{
				Endpoint:      endpoint,
				ClientID:      clientID,
				ClientSecret:  clientSecret,
				RedirectURL:   redirectURL,
				AllowInsecure: allowInsecure,
			})
			if err != nil {
				return fmt.Errorf("falha no login: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "✅ Login efetuado com sucesso!")
			fmt.Fprintf(cmd.OutOrStdout(), "   Cliente: %s (token terminado em %s)\n",
				cfg.ClientID, tokenSuffix(cfg.Token))
			return nil
		},
	}

	cmd.Flags().String("client-id", "", "ID do cliente OAuth registrado no IAM")
	cmd.Flags().String("client-secret", "", "Segredo do cliente OAuth (contas de serviço; client-credentials)")
	cmd.Flags().String("redirect-url", "http://127.0.0.1:8484/callback", "URL de callback registrada no cliente (fluxo PKCE)")
	cmd.Flags().StringP("endpoint", "u", auth.DefaultEndpoint, "URL base do Gateway da CodeArena")
	cmd.Flags().Bool("insecure", false, "PERIGO: aceita HTTP sem TLS (apenas desenvolvimento local)")
	return cmd
}

func tokenSuffix(token string) string {
	const suffixLen = 4
	if len(token) <= suffixLen {
		return "***"
	}
	return token[len(token)-suffixLen:]
}

var loginCmd = newLoginCmd()

func init() {
	rootCmd.AddCommand(loginCmd)
}
