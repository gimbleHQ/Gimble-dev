# Gimble

Gimble is a cross-platform CLI that supports:

- Linux (`amd64`, `arm64`)
- macOS (`amd64`, `arm64`)

On Linux, Gimble is distributed via APT so users can install with:

```bash
sudo apt install gimble
```

## Quick install (Linux via APT)

One-time setup:

```bash
curl -fsSL https://raw.githubusercontent.com/Saketspradhan/Gimble-dev/gh-pages/gimble-archive-keyring.gpg \
  | sudo tee /usr/share/keyrings/gimble-archive-keyring.gpg >/dev/null

echo "deb [signed-by=/usr/share/keyrings/gimble-archive-keyring.gpg] https://saketspradhan.github.io/Gimble-dev stable main" \
  | sudo tee /etc/apt/sources.list.d/gimble.list >/dev/null

sudo apt update
sudo apt install gimble
```

Start a Gimble session:

```bash
Gimble
```

Type `exit` to return to your previous shell.

## Install (macOS)

A Homebrew formula template exists at:

- `packaging/homebrew/gimble.rb`

Update formula URL + checksums for your release binaries, then users can install with Homebrew.

## What happens when you run Gimble

After install, both commands are available:

- `gimble`
- `Gimble`

Running either command opens a child interactive shell session in the current terminal, with a Gimble prompt prefix. This behaves similarly to tools that open a scoped shell session. Use `exit` to leave.

## Profile and config management

Gimble supports local user profiles (like git-style identity config).

### Config file location

- Linux: `~/.config/gimble/config.json`
- macOS: `~/Library/Application Support/gimble/config.json`

### Create your first profile

```bash
gimble profile init \
  --name "Saket Pradhan" \
  --email "saketp@umich.edu" \
  --github "Saketspradhan"
```

### Manage profiles

```bash
gimble profile list
gimble profile show
gimble profile set --profile default --email new@email.com
gimble profile use default
gimble profile delete oldprofile
```

When a profile is active and you enter Gimble, these env vars are available in-session:

- `GIMBLE_PROFILE`
- `GIMBLE_USER_NAME`
- `GIMBLE_USER_EMAIL`
- `GIMBLE_USER_GITHUB`

## Build from source

Build local binary:

```bash
make build
```

Build release binaries:

```bash
make build-linux
make build-macos
```

Artifacts are written to `dist/`.

## Build Debian packages locally

```bash
make package-deb VERSION=0.1.0
```

This creates:

- `dist/gimble_0.1.0_amd64.deb`
- `dist/gimble_0.1.0_arm64.deb`

## Release and publish workflow (maintainer)

Workflow file:

- `.github/workflows/publish-apt.yml`

On tag push (`v*`), workflow does all of this:

1. Builds Linux binaries and `.deb` packages.
2. Generates APT repo metadata (`dists/stable`, `pool/`).
3. Signs metadata (`InRelease`, `Release.gpg`).
4. Publishes APT repository to `gh-pages`.
5. Creates GitHub Release and uploads `.deb` artifacts.

### Required GitHub Actions secrets

- `APT_GPG_PRIVATE_KEY_B64`
- `APT_GPG_KEY_ID`
- `APT_GPG_PASSPHRASE`

Key setup details:

- `scripts/apt/KEY_SETUP.md`

### Publish a release

```bash
git tag v0.1.2
git push origin v0.1.2
```

## Updating Gimble (users)

```bash
sudo apt update
sudo apt upgrade gimble
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
