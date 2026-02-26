package cmd

import (
	"fmt"
	"os"

	"github.com/codearena-platform/codearena-cli/internal/auth"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Efetua o login na CodeArena",
	Run: func(cmd *cobra.Command, args []string) {
		email, _ := cmd.Flags().GetString("email")
		password, _ := cmd.Flags().GetString("password")
		endpoint, _ := cmd.Flags().GetString("endpoint")

		if email == "" || password == "" {
			fmt.Println("❌ Por favor informe o --email e a --password")
			os.Exit(1)
		}

		fmt.Printf("🔐 Autenticando %s em %s...\n", email, endpoint)

		if err := auth.Login(endpoint, email, password); err != nil {
			fmt.Printf("❌ %v\n", err)
			os.Exit(1)
		}

		fmt.Println("✅ Login efetuado com sucesso!")
	},
}

func init() {
	loginCmd.Flags().StringP("email", "e", "", "E-mail cadastrado na CodeArena")
	loginCmd.Flags().StringP("password", "p", "", "Sua senha segura")
	loginCmd.Flags().StringP("endpoint", "u", "http://localhost:3000", "URL do Gateway da CodeArena")
	rootCmd.AddCommand(loginCmd)
}
