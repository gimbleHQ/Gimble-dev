# Command reference

Commands available from your system shell vs. **inside** an active Gimble session differ: use `gimble` (and `gimble …` subcommands) to start and configure; use `gim …` only after you have started a session.

## Quick lookup

| When | Command | What it does |
|------|---------|----------------|
| Shell | `gimble` / `gimble session` | Start Gimble shell session |
| Shell | `gimble --version` | Print CLI version |
| Shell | `gimble setup` | First-time setup wizard |
| Shell | `gimble keys` | Update OpenAI / Groq / Nebius API keys |
| Shell | `gimble profile …` | Manage profiles (see [Profile commands](#profile-commands)) |
| Session | `gim chat` | Start Gimble Cloud session + log uploader |
| Session | `gim keys` | Update OpenAI / Groq / Nebius API keys |
| Session | `gim profile` | Show active profile details |
| Session | `gim exit` | Stop uploader and exit session |

---

## Top-level (`gimble`)

Run these from a normal terminal (not necessarily inside Gimble yet).

```bash
gimble                     # Start Gimble shell session
gimble session             # Same as above
gimble --version           # Print version
gimble setup               # Run first-time setup wizard
gimble keys                # Update OpenAI / Groq / Nebius API keys
gimble profile <command>   # Manage Gimble profiles
gimble profile             # Show active profile details
```

**Help**

```bash
gimble --help
```

---

## Inside a session (`gim`)

After `gimble` or `gimble session`, use the `gim` prefix:

```bash
gim chat                   # Start Gimble Cloud session + log uploader
gim keys                   # Update OpenAI / Groq / Nebius API keys
gim profile                # Show active profile details
gim exit                   # Exit the active Gimble session
```

---

## Profile commands

Use `gimble profile …` from your normal shell. Inside a session, `gim profile` only **shows** the active profile; creating, switching, and deleting profiles uses `gimble profile` from outside (or from another terminal).

**Summary**

| Subcommand | What it does |
|------------|----------------|
| *(no subcommand)* | Show active profile details |
| `init` | Create a profile (`--name`, `--email`, `--github`; optional `--provider` `github` or `gitlab`; optional `--profile`) |
| `set` | Update a profile (`--profile` plus optional `--name`, `--email`, `--github`, `--provider`) |
| `list` | List profiles |
| `show` | Show a profile (`[profile]` defaults to active) |
| `use` | Set active profile |
| `delete` | Remove a profile |

**Exact forms** (match `gimble --help`):

```text
gimble profile init --name <name> --email <email> --github <github> [--provider github|gitlab] [--profile <name>]
gimble profile set --profile <name> [--name <name>] [--email <email>] [--github <github>] [--provider github|gitlab]
gimble profile list
gimble profile show [profile]
gimble profile use <profile>
gimble profile delete <profile>
```

**Examples** (replace placeholders):

```bash
gimble profile init --name "Ada" --email "ada@example.com" --github "adal"
gimble profile set --profile "Ada" --email "new@example.com"
gimble profile list
gimble profile show
gimble profile use "team"
gimble profile delete "old-profile"
```

---

## Tips

- Run `gimble --help` or `gimble <subcommand> --help` for flags and edge cases not listed here.
- Before scripting against the CLI, pin behavior with `gimble --version` so automation matches the installed binary.
- Start cloud chat and log upload with `gim chat` **after** you are in a session; exit cleanly with `gim exit`.
