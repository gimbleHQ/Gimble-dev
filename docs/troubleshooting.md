# Troubleshooting

Quick fixes for common first-run problems. Most issues come down to **PATH**, **file permissions**, or **network access**.

| Topic | Section |
|-------|---------|
| Shell can’t find `gimble` | [Command not found](#command-not-found) |
| Config / log writes fail | [Permission denied](#permission-denied-config-or-logs) |
| `brew` / install paths | [Homebrew errors](#homebrew-install-errors) |
| Linux package keyring | [APT / keyring](#apt-or-keyring-issues-linux) |
| Downloads or cloud uploads | [Network / proxy](#network-proxy-or-cloud-uploads) |
| Setup never finishes | [Installer hangs](#installer-hangs-or-incomplete-setup) |

---

## Command not found

**Symptom:** `gimble: command not found`

**Try this:**

- If you used the install script, confirm the binary is on your `PATH`. Typical locations:

  | Platform | Common path |
  |----------|-------------|
  | macOS (Intel Homebrew) | `/usr/local/bin` |
  | macOS (Apple Silicon Homebrew) | `/opt/homebrew/bin` |
  | Linux | Your distro’s `bin` or the path printed by the installer |

- If you installed via Homebrew, run `brew list gimble` and ensure `$(brew --prefix)/bin` is on your `PATH`.

---

## Permission denied (config or logs)

**Symptom:** Errors when writing config, state, or logs.

**Try this:**

- Check ownership and permissions on your home directory and Gimble’s config locations. Example (macOS + Linux paths):

```bash
# change ownership to your user
sudo chown -R "$(whoami)" "$HOME/Library/Application Support/gimble"
sudo chown -R "$(whoami)" "$HOME/.config/gimble"
```

- Do not run the CLI as **root** for normal use — it can create root-owned files that block your user later.

---

## Homebrew install errors

**Symptom:** Permission errors, blocked directories, or failed `brew` steps.

**Try this:**

- Run `brew doctor` and apply its suggestions.
- Ensure Homebrew’s prefix directories are writable by your user, or use the repair commands Homebrew prints.

---

## APT or keyring issues (Linux)

**Symptom:** APT or GPG keyring step failed during install.

**Try this:**

- Re-run the keyring step with `sudo` and confirm the keyring file exists, for example at `/usr/share/keyrings/gimble-archive-keyring.gpg`, then run `sudo apt update`.

---

## Network, proxy, or cloud uploads

**Symptom:** Install curl fails, or uploads / cloud features time out.

**Try this:**

- If you use a proxy, set `HTTP_PROXY` and `HTTPS_PROXY` in the shell where you run Gimble.
- Confirm DNS and outbound HTTPS are allowed to `raw.githubusercontent.com` and to the cloud API host your setup uses.

---

## Installer hangs or incomplete setup

**Symptom:** Script or wizard never returns.

**Try this:**

- Run `gimble --version` directly to verify the binary executes.
- Check disk space and that config directories are writable (see [Permission denied](#permission-denied-config-or-logs)).

---

## Still need help?

- Command syntax: [Command reference](commands.md) and `gimble --help` / `gimble <subcommand> --help`.
- Environment and config: [Environment and local config](env.md).

If something still fails, open an issue with **OS** (macOS or Linux), **`gimble --version`**, and the **exact error** (copy stderr if you can).
