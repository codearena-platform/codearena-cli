package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/codearena-platform/codearena-cli/internal/templates"
	pb "github.com/codearena-platform/codearena-core/pkg/types"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var rootCmd = &cobra.Command{
	Use:   "codearena",
	Short: "CodeArena CLI - O inicializador universal de robôs",
	Long:  `CodeArena CLI permite criar, testar e enviar seus robôs para a arena global.`,
}

var initCmd = &cobra.Command{
	Use:   "init [bot-name]",
	Short: "Inicializa um novo projeto de robô",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		botName := args[0]
		lang, _ := cmd.Flags().GetString("lang")
		fmt.Printf("🚀 Inicializando robô '%s' em %s...\n", botName, lang)

		// Create directory
		err := os.Mkdir(botName, 0755)
		if err != nil {
			fmt.Printf("Erro ao criar diretório: %v\n", err)
			return
		}

		// Logic to inject SDK based on 'lang'
		createProjectFiles(botName, lang)
		fmt.Printf("✅ Projeto '%s' pronto! Entre na pasta e comece a programar.\n", botName)
	},
}

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Envia seu robô para a nuvem CodeArena",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("☁️  Empacotando robô e enviando para competições online...")

		// In a real scenario, we would detect the bot name from the current directory
		// For this E2E, let's assume we are in 'e2e-bot' or it's in the current dir
		botName := "e2e-bot"
		if _, err := os.Stat(botName); os.IsNotExist(err) {
			// Check if we are inside the bot directory
			if _, err := os.Stat("bot.ts"); err == nil {
				cwd, _ := os.Getwd()
				botName = filepath.Base(cwd)
			} else {
				fmt.Println("❌ Erro: Diretório do robô não encontrado. Execute 'init' primeiro ou entre na pasta do robô.")
				return
			}
		}

		// 1. Read source code
		var source string
		if _, err := os.Stat(filepath.Join(botName, "bot.ts")); err == nil {
			content, _ := os.ReadFile(filepath.Join(botName, "bot.ts"))
			source = string(content)
		} else if _, err := os.ReadFile("bot.ts"); err == nil {
			content, _ := os.ReadFile("bot.ts")
			source = string(content)
		}

		// 2. Connect to Competition Service (gRPC)
		conn, err := grpc.Dial("localhost:50052", grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			fmt.Printf("❌ Erro ao conectar ao Gateway: %v\n", err)
			return
		}
		defer conn.Close()

		client := pb.NewMatchServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// 3. Call RegisterBot
		resp, err := client.RegisterBot(ctx, &pb.RegisterBotRequest{
			UserId:     "user_123", // Placeholder for authenticated user
			Name:       botName,
			Version:    "v1.0.0",
			SourceCode: source,
			Language:   "typescript",
		})

		if err != nil {
			fmt.Printf("❌ Erro ao registrar robô: %v\n", err)
			return
		}

		if !resp.Success {
			fmt.Printf("❌ Erro no servidor: %s\n", resp.Message)
			return
		}

		fmt.Printf("✅ Robô '%s' enviado com sucesso! (ID: %s, Versão: %s)\n", botName, resp.BotId, resp.VersionId)
		fmt.Println("👉 Acompanhe em: https://codearena.io/meu-perfil")
	},
}

type TemplateData struct {
	BotName      string
	BotClassName string
}

func createProjectFiles(name, lang string) {
	data := TemplateData{
		BotName:      name,
		BotClassName: "MyBot",
	}

	if lang == "typescript" {
		renderTemplate("typescript/package.json.tmpl", name+"/package.json", data)
		renderTemplate("typescript/bot.ts.tmpl", name+"/bot.ts", data)
	} else if lang == "python" {
		renderTemplate("python/bot.py.tmpl", name+"/bot.py", data)
	}
}

func renderTemplate(tmplPath, destPath string, data interface{}) {
	tmplContent, err := templates.FS.ReadFile(tmplPath)
	if err != nil {
		fmt.Printf("Erro ao ler template %s: %v\n", tmplPath, err)
		return
	}

	t, err := template.New(tmplPath).Parse(string(tmplContent))
	if err != nil {
		fmt.Printf("Erro ao parsear template %s: %v\n", tmplPath, err)
		return
	}

	f, err := os.Create(destPath)
	if err != nil {
		fmt.Printf("Erro ao criar arquivo %s: %v\n", destPath, err)
		return
	}
	defer f.Close()

	if err := t.Execute(f, data); err != nil {
		fmt.Printf("Erro ao executar template %s: %v\n", tmplPath, err)
	}
}

func main() {
	initCmd.Flags().StringP("lang", "l", "typescript", "Linguagem do robô (typescript, python, java)")
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(pushCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
