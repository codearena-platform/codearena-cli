package cmd

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/codearena-platform/codearena-cli/internal/auth"
	"github.com/codearena-platform/codearena-cli/internal/packager"
	"github.com/codearena-platform/codearena-cli/internal/project"
	pb "github.com/codearena-platform/codearena-core/pkg/api/v1"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"gopkg.in/yaml.v3"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Envia seu robô para a nuvem CodeArena",
	Run: func(cmd *cobra.Command, args []string) {
		gateway, _ := cmd.Flags().GetString("gateway")

		// 1. Verify Authentication
		cfg, err := auth.ReadConfig()
		if err != nil {
			fmt.Printf("❌ %v\n", err)
			os.Exit(1)
		}

		// 2. Read Project Manifest
		manifestData, err := os.ReadFile("codearena.yaml")
		if err != nil {
			fmt.Println("❌ Erro: 'codearena.yaml' não encontrado. Você está na raiz do seu robô?")
			os.Exit(1)
		}

		var manifest project.Manifest
		if err := yaml.Unmarshal(manifestData, &manifest); err != nil {
			fmt.Printf("❌ Erro ao ler 'codearena.yaml': %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("☁️  Empacotando robô %s (%s) e enviando...\n", manifest.Name, manifest.Version)

		// 3. Zip code
		cwd, _ := os.Getwd()
		zipBytes, err := packager.ZipDir(cwd)
		if err != nil {
			fmt.Printf("❌ Erro ao empacotar: %v\n", err)
			os.Exit(1)
		}

		// 4. Encode as Base64 (to send safely through existing SourceCode protobuf string field)
		// Or send straight bytes if you change the protobuf field. For now, base64 is safest.
		zipBase64 := base64.StdEncoding.EncodeToString(zipBytes)

		// 5. Connect to Match/Competition gRPC
		// Notice how we could also use the HTTP Gateway we made earlier. For this cli, let's keep gRPC if requested
		conn, err := grpc.Dial(gateway, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			fmt.Printf("❌ Erro ao conectar ao Gateway: %v\n", err)
			return
		}
		defer conn.Close()

		client := pb.NewMatchServiceClient(conn)

		// Attach token to gRPC metadata
		md := metadata.Pairs("authorization", "Bearer "+cfg.Token)
		ctx := metadata.NewOutgoingContext(context.Background(), md)
		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()

		// 6. Push
		resp, err := client.RegisterBot(ctx, &pb.RegisterBotRequest{
			UserId:     cfg.Email, // Identity from config logic
			Name:       manifest.Name,
			Version:    manifest.Version,
			SourceCode: zipBase64, // Sending base64 zip payload
			Language:   manifest.Language,
		})

		if err != nil {
			fmt.Printf("❌ Erro ao registrar robô: %v\n", err)
			return
		}

		if !resp.Success {
			fmt.Printf("❌ Erro no servidor: %s\n", resp.Message)
			return
		}

		fmt.Printf("✅ Robô '%s' enviado com sucesso! (ID: %s, Versão: %s)\n", manifest.Name, resp.BotId, resp.VersionId)
		fmt.Println("👉 Acompanhe em: https://codearena.io/meu-perfil")
	},
}

func init() {
	pushCmd.Flags().StringP("gateway", "g", "localhost:50052", "Endereço gRPC do Gateway ou Competition Server")
	rootCmd.AddCommand(pushCmd)
}
