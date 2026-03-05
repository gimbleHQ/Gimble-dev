# Gimble

Gimble is a cross-platform CLI for Linux and macOS.

## Install

### macOS (Homebrew)

```bash
brew tap saketspradhan/gimble https://github.com/Saketspradhan/Gimble-dev
brew install gimble
```

Start Gimble:

```bash
gimble
```

### Linux (APT)

One-time repository setup:

```bash
curl -fsSL https://raw.githubusercontent.com/Saketspradhan/Gimble-dev/gh-pages/gimble-archive-keyring.gpg \
  | sudo tee /usr/share/keyrings/gimble-archive-keyring.gpg >/dev/null

echo "deb [signed-by=/usr/share/keyrings/gimble-archive-keyring.gpg] https://saketspradhan.github.io/Gimble-dev stable main" \
  | sudo tee /etc/apt/sources.list.d/gimble.list >/dev/null
```

Install:

```bash
sudo apt update
sudo apt install gimble
```

Start Gimble:

```bash
gimble
```

## Local Chat (inside Gimble session)

`gim chat` runs a **Python** backend with two primary providers: **Groq** and **OpenAI API**.

Default selection:

- `openai/gpt-oss-120b` via Groq

Browser dropdown models:

- Groq:
  - `openai/gpt-oss-120b`
  - `openai/gpt-oss-20b`
  - `openai/gpt-oss-safeguard-20b`
  - `qwen/qwen3-32b`
  - `llama-3.1-8b-instant`
  - `llama-3.3-70b-versatile`
- OpenAI API:
  - `gpt-4o-mini`
  - `gpt-4.1-mini`
  - `gpt-4.1-nano`
- Experimental local option:
  - `GPT-Q 4K (Experimental, developer-only)`

### 1. Install the Gimble Python runtime (one-time)

From the repo root:

```bash
./python/setup_runtime.sh
```

### 2. Configure local API keys

```bash
mkdir -p "$HOME/Library/Application Support/gimble"   # macOS
# mkdir -p "$HOME/.config/gimble"                    # Linux

cat > "$HOME/Library/Application Support/gimble/chat.env" <<'ENV'
GROQ_API_KEY=<your_groq_key>
GROQ_MODEL=openai/gpt-oss-120b
OPENAI_API_KEY=<your_openai_key>
OPENAI_MODEL=gpt-4o-mini
ENV
```

### 3. Start chat

```bash
gimble
# now inside Gimble session

gim chat
```

### System prompt support

You can set explicit system prompts in two ways:

1. API field `system_prompt` in `/api/chat` payload.
2. Message prefix in chat input:

```text
System: You are terse and technical.
User: Explain mmap in one paragraph.
```

A `System: ...` prompt is persisted per selected model session and honored for future turns.

`gimble chat` is intentionally disabled outside session.

## First Run (Recommended)

Initialize your profile once:

```bash
gimble profile init \
  --name "Your Name" \
  --email "you@example.com" \
  --github "your-github"
```

Then open a session anytime:

```bash
gimble
```

Type `exit` to return to your normal shell.

## Profile and Config Management

Gimble stores profile config at:

- Linux: `~/.config/gimble/config.json`
- macOS: `~/Library/Application Support/gimble/config.json`

Useful profile commands:

```bash
gimble profile list
gimble profile show
gimble profile set --profile default --email new@email.com
gimble profile use default
gimble profile delete oldprofile
```

Inside a Gimble session, active profile data is exported as:

- `GIMBLE_PROFILE`
- `GIMBLE_USER_NAME`
- `GIMBLE_USER_EMAIL`
- `GIMBLE_USER_GITHUB`

## Build from Source

```bash
make build
```

Release binaries:

```bash
make build-linux
make build-macos
```

Artifacts are written to `dist/`.

## Build Debian Packages Locally

```bash
make package-deb VERSION=0.1.0
```

This creates:

- `dist/gimble_0.1.0_amd64.deb`
- `dist/gimble_0.1.0_arm64.deb`

## Release and Publish Workflow (Maintainer)

Workflow file:

- `.github/workflows/publish-apt.yml`

On tag push (`v*`), the workflow:

1. Builds Linux binaries and `.deb` packages.
2. Generates APT repo metadata (`dists/stable`, `pool/`).
3. Signs metadata (`InRelease`, `Release.gpg`).
4. Publishes APT repository to `gh-pages`.
5. Creates GitHub Release and uploads `.deb` artifacts.

Required GitHub Actions secrets:

- `APT_GPG_PRIVATE_KEY_B64`
- `APT_GPG_KEY_ID`
- `APT_GPG_PASSPHRASE`

Key setup details:

- `scripts/apt/KEY_SETUP.md`

Publish a release:

```bash
git tag v0.1.11
git push origin v0.1.11
```

## Updating Gimble

### Linux

```bash
sudo apt update
sudo apt upgrade gimble
```

### macOS

```bash
brew update
brew upgrade gimble
```

## Troubleshooting

### `E: ... does not have a Release file`

Your Pages-hosted APT repo is not available yet.

Check:

- `https://saketspradhan.github.io/Gimble-dev/dists/stable/Release` returns `200`
- latest `Publish APT Repository` workflow run is green
- GitHub Pages is enabled for branch `gh-pages` root

### `base64: invalid input` during signing step

`APT_GPG_PRIVATE_KEY_B64` secret is malformed.

Regenerate exactly:

```bash
gpg --armor --export-secret-keys <KEY_ID> | base64 | tr -d '\n'
```

Paste that single-line value as the secret.

### Workflow warning: missing `go.sum`

This is a cache warning from `actions/setup-go`; it does not block publishing.

## License

See `LICENSE`.
