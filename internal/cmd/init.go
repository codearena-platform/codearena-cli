package cmd

import (
	"fmt"
	"os"

	"github.com/codearena-platform/codearena-cli/internal/project"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [bot-name]",
	Short: "Inicializa um novo projeto de robô",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		botName := args[0]
		lang, _ := cmd.Flags().GetString("lang")
		fmt.Printf("🚀 Inicializando robô '%s' em %s...\n", botName, lang)

		err := project.InitProject(botName, lang)
		if err != nil {
			fmt.Printf("❌ Erro ao inicializar o projeto: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ Projeto '%s' pronto! Entre na pasta e comece a programar.\n", botName)
	},
}

func init() {
	initCmd.Flags().StringP("lang", "l", "typescript", "Linguagem do robô (typescript, python, java)")
	rootCmd.AddCommand(initCmd)
}
