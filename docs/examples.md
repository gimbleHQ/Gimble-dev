# Examples — short workflows

## Install and verify

Install the latest release (recommended):

```bash
curl -fsSL https://raw.githubusercontent.com/Saketspradhan/Gimble-dev/main/scripts/install_latest.sh | bash

gimble --version
```

## Start a session and chat in cloud mode

1. Start a local Gimble session:

```bash
gimble
```

2. Inside the session, start cloud chat (this will create a cloud session and start the uploader):

```bash
gim chat
```

3. The CLI prints a session URL such as `https://chat.gimble.dev/<username>/<session_id>`; open it in a browser to view the conversation.

## Create and use a profile

```bash
gimble profile init --name "Saket" --email saket@example.com --github saketspradhan

# list available profiles
gimble profile list

# switch to a profile
gimble profile use default
```

## Install via package manager

- Homebrew (macOS):

```bash
brew tap saketspradhan/gimble https://github.com/Saketspradhan/Gimble-dev
brew install gimble
```

- APT (Linux): follow the one-time keyring setup then:

```bash
sudo apt update
sudo apt install gimble
```


These short examples cover the most common developer and user flows; see `commands.md` for more detail.
