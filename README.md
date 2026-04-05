# Gimble CLI

[![MIT License](https://img.shields.io/badge/license-MIT-brightgreen)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-111111)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/Saketspradhan/Gimble-dev?display_name=tag)](https://github.com/Saketspradhan/Gimble-dev/releases/latest)

Gimble is a free, open-source CLI for debugging physical systems. It captures terminal and log context, ingests live telemetry and system state—so engineers can get answers grounded in real events and fix issues faster without digging through thousands of log lines.

- Capture live terminal and log context as you work
- Open a live browser session you can share
- Get answers with evidence, not hallucinations or guesswork

## Quickstart

Install (Linux + macOS):

```bash
curl -fsSL https://raw.githubusercontent.com/Saketspradhan/Gimble-dev/main/scripts/install_latest.sh | bash
```

Then finish **first-time setup** (the installer or `gimble setup` will guide you).

**Start a session** (normal terminal):

```bash
gimble
```

**Cloud chat + log upload** (run inside that session):

```bash
gim chat
```

Gimble CLI connects to a hosted Gimble Cloud companion that powers chat and evidence retrieval.

![Gimble CLI to Cloud flow (terminal + live UI)](docs/assets/gimble-story.png)

## How it works

```mermaid
flowchart LR
  CLI["Gimble CLI on local machine"] -->|" sanitized session logs "| Cloud["Gimble Cloud (hosted)"]
  Cloud -->|" live chat + evidence "| UI["Browser UI"]
```

- The CLI captures session activity and uploads sanitized logs.
- Gimble Cloud turns that context into a live, queryable and shareable browser session.
- Every answer is grounded with evidence from your session history.

---

## Usage

Commands depend on **where** you run them: your normal shell (`gimble …`) vs **inside** an active Gimble session (`gim …`).

### Shell (`gimble`)

| Command | What it does |
|--------|----------------|
| `gimble` / `gimble session` | Start a Gimble shell session |
| `gimble --version` | Print the installed version |
| `gimble setup` | Run the first-time setup wizard |
| `gimble keys` | Set OpenAI, Groq, or Nebius API keys |
| `gimble profile` | Show the active profile; use `gimble profile <subcommand>` to create, switch, or edit profiles |

### Inside a session (`gim`)

| Command | What it does |
|--------|----------------|
| `gim chat` | Start Gimble Cloud chat and the log uploader |
| `gim keys` | Update API keys without leaving the session |
| `gim profile` | Show the active profile |
| `gim exit` | Stop the uploader and leave the session |

Full syntax (especially profiles), flags, and examples: **[command reference](docs/commands.md)**. Use `gimble --help` for the exact text your build ships with.

---

## Documentation

| Doc | Contents |
|-----|----------|
| **[Command reference](docs/commands.md)** | All commands, profile subcommands, examples |
| **[Environment & local config](docs/env.md)** | Config paths, `chat.env`, proxies, logs |
| **[Troubleshooting](docs/troubleshooting.md)** | PATH, permissions, Homebrew, APT, network |

---

## Support

**Issues & feature requests** — [GitHub Issues](https://github.com/Saketspradhan/Gimble-dev/issues).

**Email** — [gimble256@gmail.com](mailto:gimble256@gmail.com).

## License

MIT. See [LICENSE](LICENSE).
