# 💻 CodeArena CLI

O **CodeArena CLI** é a sua ferramenta principal para criar, testar e enviar seus robôs para a arena global do CodeArena.

## 🚀 Como Usar

O CLI possui comandos simples para você começar rapidamente a desenvolver e publicar seus robôs.

### 1. Inicializar um Novo Robô

Para inicializar um novo projeto de robô a partir de templates, use o comando `init`:

```bash
codearena init <nome-do-robo> --lang <linguagem>
```

**Parâmetros:**
- `<nome-do-robo>`: O nome do diretório e do projeto a ser criado.
- `--lang` ou `-l` (Opcional): A linguagem de programação do seu bot. As linguagens suportadas são `typescript`, `python` e `java`. O padrão é `typescript`.

**Exemplo:**
```bash
codearena init meu-primeiro-robo --lang typescript
```

Isso criará uma pasta `meu-primeiro-robo` com a estrutura inicial necessária para começar a programar a lógica do seu bot.

### 2. Enviar Robô para a Nuvem CodeArena

Ao terminar de programar, você pode empacotar e enviar (fazer "push") do seu robô para os nossos servidores usando:

```bash
codearena push
```

**Como Funciona:**
- Execute este comando de dentro do diretório do seu robô, ou assegure-se de que um robô válido como `e2e-bot` (ou a pasta atual) existe no local.
- O CLI lerá o código-fonte (por exemplo, `bot.ts`) e o enviará para o Gateway do CodeArena via gRPC.
- Se o envio for bem-sucedido, o servidor retornará o ID do robô e a versão registrada.

## ⚙️ Instalação (Desenvolvimento Local)

Se você quiser compilar ou instalar o CLI localmente:

```bash
# Baixar dependências
go mod tidy

# Compilar
go build -o codearena main.go

# Instalar globalmente (Opcional)
go install
```

## 🏗️ Estrutura do Projeto

- `main.go`: O ponto de entrada principal do CLI utilizando o framework [Cobra](https://github.com/spf13/cobra).
- `internal/templates`: Diretório que armazena os esqueletos de projetos (templates) que são injetados durante o comando `init`.
- `e2e-bot/`: Diretório de exemplo ou utilitário usado frequentemente para testes ponta a ponta.
