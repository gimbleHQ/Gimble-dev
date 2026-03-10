# Troubleshooting — common first-run issues

This page lists quick fixes for problems you may encounter on first run.

1) "gimble: command not found"

- If you used the curl install script, ensure the installer put the binary on your `PATH`. Common paths:
  - `/usr/local/bin` (Homebrew on Intel macs)
  - `/opt/homebrew/bin` (Homebrew on Apple Silicon)

- If installed via Homebrew, run `brew list gimble` and ensure `brew --prefix` is on your PATH.

2) Permission denied when writing config or logs

- Check ownership and permissions of your home directory and the Gimble config path. Example fix:

```bash
# change ownership to your user
sudo chown -R $(whoami) "$HOME/Library/Application Support/gimble"
sudo chown -R $(whoami) "$HOME/.config/gimble"
```

- Avoid running the CLI as root — it can create files with root ownership that block the normal user.

3) Homebrew install errors (permissions, blocked directories)

- Run `brew doctor` and follow its recommendations. Ensure Homebrew's install directories are writable by your user or use Homebrew's recommended fix commands.

4) APT / keyring issues (Linux)

- If the APT keyring step failed, re-run the keyring curl command with `sudo` and verify the file exists at `/usr/share/keyrings/gimble-archive-keyring.gpg`, then run `sudo apt update`.

5) Network / proxy problems (downloads or cloud uploads fail)

- Ensure `HTTP_PROXY` / `HTTPS_PROXY` are set in your shell environment if you're behind a proxy.
- Verify DNS and outbound TLS traffic are allowed to `raw.githubusercontent.com`, `saketspradhan.github.io`, and the cloud API base you are using.

6) Installer hangs or interactive setup doesn't complete

- Run the binary directly with `gimble --version` to confirm the executable runs.
- Check file system permissions and free disk space.

7) Want more help?

- Use `gimble --help` and `gimble <subcommand> --help` for command-specific flags.
- If you still see failures, capture the exact errors and open an issue in the repository with the platform (macOS/Linux), Gimble version, and a copy of relevant stderr output.
