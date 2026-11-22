package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Overridden at build time by GoReleaser (-X ...version=...).
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Exibe a versão da CodeArena CLI",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "codearena %s (commit %s, built %s)\n", version, commit, date)
			return err
		},
	}
}

var versionCmd = newVersionCmd()

func init() {
	rootCmd.AddCommand(versionCmd)
}
