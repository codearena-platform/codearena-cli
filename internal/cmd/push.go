package cmd

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/codearena-platform/codearena-cli/internal/auth"
	"github.com/codearena-platform/codearena-cli/internal/packager"
	"github.com/codearena-platform/codearena-cli/internal/project"
	pb "github.com/codearena-platform/codearena-core/pkg/api/v1"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"gopkg.in/yaml.v3"
)

func newPushCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push",
		Short: "Envia seu robô para a nuvem CodeArena",
		RunE: func(cmd *cobra.Command, args []string) error {
			gateway, err := cmd.Flags().GetString("gateway")
			if err != nil {
				return err
			}
			allowInsecure, err := cmd.Flags().GetBool("insecure")
			if err != nil {
				return err
			}

			cfg, err := auth.ReadConfig()
			if err != nil {
				return err
			}

			manifestData, err := os.ReadFile("codearena.yaml")
			if err != nil {
				return fmt.Errorf("ler 'codearena.yaml' (você está na raiz do seu robô?): %w", err)
			}
			var manifest project.Manifest
			if err := yaml.Unmarshal(manifestData, &manifest); err != nil {
				return fmt.Errorf("interpretar 'codearena.yaml': %w", err)
			}

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("obter diretório atual: %w", err)
			}
			zipBytes, err := packager.ZipDir(cwd)
			if err != nil {
				return fmt.Errorf("empacotar robô: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "☁️  Empacotando robô %s (%s) e enviando...\n", manifest.Name, manifest.Version)
			return pushBot(cmd.Context(), gateway, allowInsecure, cfg, &manifest, zipBytes, cmd.OutOrStdout())
		},
	}

	cmd.Flags().StringP("gateway", "g", "localhost:50052", "Endereço gRPC do Gateway ou Competition Server")
	cmd.Flags().Bool("insecure", false, "PERIGO: usa gRPC sem TLS (apenas desenvolvimento local)")
	return cmd
}

// transportCredentials returns TLS credentials unless the caller explicitly
// opts into plaintext for local development.
func transportCredentials(allowInsecure bool) (credentials.TransportCredentials, error) {
	if allowInsecure {
		return insecure.NewCredentials(), nil
	}
	return credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12}), nil
}

// pushBot registers the packaged bot. The user id is deliberately left empty:
// the competition service derives it from the verified bearer token.
func pushBot(ctx context.Context, gateway string, allowInsecure bool, cfg *auth.Config, manifest *project.Manifest, zipBytes []byte, out io.Writer) error {
	if cfg == nil || cfg.Token == "" {
		return fmt.Errorf("token de acesso ausente; execute 'codearena login'")
	}
	if manifest == nil || manifest.Name == "" {
		return fmt.Errorf("manifesto do robô inválido")
	}

	creds, err := transportCredentials(allowInsecure)
	if err != nil {
		return err
	}
	conn, err := grpc.NewClient(gateway, grpc.WithTransportCredentials(creds))
	if err != nil {
		return fmt.Errorf("conectar ao Gateway: %w", err)
	}
	defer conn.Close()

	md := metadata.Pairs("authorization", "Bearer "+cfg.Token)
	callCtx := metadata.NewOutgoingContext(ctx, md)
	callCtx, cancel := context.WithTimeout(callCtx, 15*time.Second)
	defer cancel()

	resp, err := pb.NewMatchServiceClient(conn).RegisterBot(callCtx, &pb.RegisterBotRequest{
		// UserId is intentionally omitted: the server derives it from the token.
		Name:       manifest.Name,
		Version:    manifest.Version,
		SourceCode: base64.StdEncoding.EncodeToString(zipBytes),
		Language:   manifest.Language,
	})
	if err != nil {
		return fmt.Errorf("registrar robô: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("servidor rejeitou o robô: %s", resp.Message)
	}

	fmt.Fprintf(out, "✅ Robô '%s' enviado com sucesso! (ID: %s, Versão: %s)\n",
		manifest.Name, resp.BotId, resp.VersionId)
	return nil
}

var pushCmd = newPushCmd()

func init() {
	rootCmd.AddCommand(pushCmd)
}
