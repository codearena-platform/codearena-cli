package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "codearena",
	Short: "CodeArena CLI - O inicializador universal de robôs",
	Long:  `CodeArena CLI permite criar, testar e enviar seus robôs para a arena global.`,
}

// Execute is the main entry point for Cobra commands
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Root flags and config initialization can go here
}
