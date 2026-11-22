# 💻 CodeArena CLI

The **CodeArena CLI** is your primary tool for creating, testing, and pushing your robots to the global CodeArena arena.

## 🚀 How to Use

The CLI has simple commands to get you started quickly developing and publishing your robots.

### 1. Initialize a New Robot

To initialize a new robot project from templates, use the `init` command:

```bash
codearena init <robot-name> --lang <language>
```

**Parameters:**
- `<robot-name>`: The name of the directory and project to be created.
- `--lang` or `-l` (Optional): The programming language of your bot. Supported languages are `typescript` and `python`. The default is `typescript`.

**Example:**
```bash
codearena init my-first-robot --lang typescript
```

This will create a `my-first-robot` folder with the initial structure needed to start programming your bot's logic.

### 2. Push Robot to the CodeArena Cloud

When you're done programming, you can package and push your robot to our servers using:

```bash
codearena push
```

**How it Works:**
- Run this command from within your robot's directory, or ensure a valid robot like `e2e-bot` (or the actual folder) exists there.
- The CLI will read the source code (e.g., `bot.ts`) and send it to the CodeArena Gateway via gRPC.
- If the push is successful, the server will return the robot ID and the registered version.

### 3. Authenticate

Login uses the IAM OAuth flows through the gateway (default `https://localhost:8080`):

```bash
# Interactive users (PKCE, opens the browser)
codearena login --client-id <oauth-client-id> --redirect-url http://127.0.0.1:8484/callback

# Service accounts (client credentials)
codearena login --client-id <oauth-client-id> --client-secret <oauth-client-secret>
```

TLS is always used unless `--insecure` is passed explicitly for local development.
The bearer token is stored in `~/.codearena/config.json` with `0600` permissions.
`push` uses TLS for gRPC as well (`--insecure` is the local-only opt-out) and never
sends a client-supplied user id: the server derives the identity from the token.

## ⚙️ Installation (Local Development)

If you want to compile or install the CLI locally:

```bash
# Download dependencies
go mod tidy

# Compile
go build -o codearena ./cmd/codearena

# Install globally (Optional)
go install
```

## 🏗️ Project Structure

- `cmd/codearena/main.go`: The main entry point of the CLI using the [Cobra](https://github.com/spf13/cobra) framework.
- `internal/templates`: Directory that stores project skeletons (templates) injected during the `init` command.
- `codearena version`: Prints the build version, commit and date (populated by GoReleaser).
- `e2e-bot/`: Example or utility directory frequently used for end-to-end testing.
