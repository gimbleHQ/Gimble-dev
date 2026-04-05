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

and complete the initial setup.

After installation and the initial setup, you can initiate a session using 

```bash
gimble 
```

Once inside a local Gimble session, you can create a queryable browser chat interface through

```bash
gim chat
```

Gimble CLI connects to a hosted Gimble Cloud companion that powers chat and evidence retrieval.

![Gimble CLI to Cloud flow (terminal + live UI)](docs/assets/gimble-story.png)

## How it works

```mermaid
flowchart LR
  CLI["Gimble CLI on local machine"] -->|"sanitized session logs"| Cloud["Gimble Cloud (hosted)"]
  Cloud -->|"live chat + evidence"| UI["Browser UI"]
```

- The CLI captures session activity and uploads sanitized logs.
- Gimble Cloud turns that context into a live, queryable and shareable browser session.
- Every answer is grounded with evidence from your session history.

## Usage

Everyday commands:

- `gimble` - start a Gimble session
- `gimble setup` - run first-time setup wizard again

- `gim chat` - start cloud chat and uploader (inside a session)
- `gim exit` - stop uploader and exit session
- `gim keys` - set OpenAI, Groq, or Nebius API keys
- `gim profile` - show logged in profile

Profiles (team and identity settings):

- Use `gimble profile ...` commands. See the docs for details.

## Docs

- [Command reference](docs/commands.md)
- [Examples](docs/examples.md)
- [Environment and local config](docs/env.md)
- [Troubleshooting](docs/troubleshooting.md)

## Support

If you hit an issue or have a feature request, please [open a GitHub issue](https://github.com/Saketspradhan/Gimble-dev/issues). You can also reach us at [gimble256@gmail.com](mailto:gimble256@gmail.com).

## License

MIT. See [LICENSE](LICENSE).
