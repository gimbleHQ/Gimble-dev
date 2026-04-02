# Commands — Expanded reference

This file collects the most-used Gimble commands and short examples.

## Start / session

- Start a shell session:

  gimble
  gimble session

- Run first-time setup wizard:

  gimble setup

## Chat / cloud

- Start cloud chat session with background uploader:

  gim chat

- Exit the session and stop uploader:

  gim exit

- Show active profile:

  gim profile

## Key & profile management

- Update API keys (OpenAI, Groq):

  gimble keys

- Profile operations:

  gimble profile
  gimble profile init --name <name> --email <email> --github <github> [--provider github|gitlab] [--profile <name>]
  gimble profile set --profile <name> [--name <name>] [--email <email>] [--github <github>] [--provider github|gitlab]
  gimble profile list
  gimble profile show [profile]
  gimble profile use <profile>
  gimble profile delete <profile>

## Help & version

- Show help for top-level commands:

  gimble --help

- Show version:

  gimble --version


Tips
- Use `--help` after any subcommand for context-specific flags (for example: `gimble profile --help`).
- When scripting, check `gimble --version` before running automated flows to ensure compatibility.
