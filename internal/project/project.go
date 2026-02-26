package project

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/codearena-platform/codearena-cli/internal/templates"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type TemplateData struct {
	BotName      string
	BotClassName string
}

type Manifest struct {
	ID       string `yaml:"id"`
	Name     string `yaml:"name"`
	Version  string `yaml:"version"`
	Language string `yaml:"language"`
}

// InitProject sets up a new bot directory with a manifest and appropriate language templates
func InitProject(name, lang string) error {
	// Create directory
	if err := os.Mkdir(name, 0755); err != nil {
		return fmt.Errorf("falha ao criar o diretório: %w", err)
	}

	data := TemplateData{
		BotName:      name,
		BotClassName: "MyBot",
	}

	// Create manifest codearena.yaml
	manifest := Manifest{
		ID:       uuid.NewString(), // Generates a new unique ID for the bot
		Name:     name,
		Version:  "1.0.0",
		Language: lang,
	}

	manifestData, err := yaml.Marshal(&manifest)
	if err != nil {
		return fmt.Errorf("falha ao criar arquivo de manifesto: %w", err)
	}

	err = os.WriteFile(filepath.Join(name, "codearena.yaml"), manifestData, 0644)
	if err != nil {
		return fmt.Errorf("falha ao salvar o manifesto: %w", err)
	}

	// Generate language specific templates
	if lang == "typescript" {
		renderTemplate("typescript/package.json.tmpl", filepath.Join(name, "package.json"), data)
		renderTemplate("typescript/bot.ts.tmpl", filepath.Join(name, "bot.ts"), data)
	} else if lang == "python" {
		renderTemplate("python/bot.py.tmpl", filepath.Join(name, "bot.py"), data)
	} else {
		return fmt.Errorf("linguagem não suportada no momento: %s", lang)
	}

	return nil
}

func renderTemplate(tmplPath, destPath string, data interface{}) {
	tmplContent, err := templates.FS.ReadFile(tmplPath)
	if err != nil {
		fmt.Printf("Erro ao ler template %s: %v\n", tmplPath, err)
		return
	}

	t, err := template.New(filepath.Base(tmplPath)).Parse(string(tmplContent))
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
