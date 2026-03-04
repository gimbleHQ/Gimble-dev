# gimble

`gimble` is a cross-platform CLI with first-class Linux and macOS support.

## Current support

- Linux: `amd64`, `arm64`
- macOS: `amd64`, `arm64`

## Build locally

```bash
make build
```

## Build release binaries

```bash
make build-linux
make build-macos
```

Artifacts are created in `dist/`.

## Build Debian packages

```bash
make package-deb VERSION=0.1.0
```

This creates:

- `dist/gimble_0.1.0_amd64.deb`
- `dist/gimble_0.1.0_arm64.deb`

## APT repo publishing workflow (GitHub Pages)

A workflow is included at `.github/workflows/publish-apt.yml`.

### What it does

- Builds Linux binaries
- Builds `.deb` packages
- Generates APT metadata under `dists/stable` and `pool/`
- Signs `Release` as `InRelease` and `Release.gpg`
- Publishes the repo to the `gh-pages` branch
- On tag pushes (`v*`), creates a GitHub Release and uploads `.deb` artifacts

### Required GitHub secrets

- `APT_GPG_PRIVATE_KEY_B64`: base64-encoded private key for repo signing
- `APT_GPG_KEY_ID`: GPG key ID or fingerprint
- `APT_GPG_PASSPHRASE`: passphrase for the private key

### Trigger publishing

- Push a tag like `v0.1.0`, or
- Run the `Publish APT Repository` workflow manually and provide `version`

## User install commands (`sudo apt install gimble`)

```bash
curl -fsSL https://raw.githubusercontent.com/Saketspradhan/Gimble-dev/gh-pages/gimble-archive-keyring.gpg \
  | sudo tee /usr/share/keyrings/gimble-archive-keyring.gpg >/dev/null

echo "deb [signed-by=/usr/share/keyrings/gimble-archive-keyring.gpg] https://saketspradhan.github.io/Gimble-dev stable main" \
  | sudo tee /etc/apt/sources.list.d/gimble.list >/dev/null

sudo apt update
sudo apt install gimble
```

## macOS install path

A Homebrew formula template is included at `packaging/homebrew/gimble.rb`.
Point it at release binaries + checksums, then users can install with Homebrew.

Detailed key creation steps are in `scripts/apt/KEY_SETUP.md`.
