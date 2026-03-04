# APT signing key setup

Create a dedicated signing key for your APT repo and add the required GitHub secrets.

## 1) Generate a GPG key (local machine)

```bash
gpg --full-generate-key
```

Use RSA 4096 and set a passphrase.

## 2) Get your key ID / fingerprint

```bash
gpg --list-secret-keys --keyid-format LONG
```

Use the long key ID or full fingerprint as `APT_GPG_KEY_ID`.

## 3) Export private key and base64 encode

```bash
gpg --armor --export-secret-keys <KEY_ID> | base64 | tr -d '\n'
```

Store this value as `APT_GPG_PRIVATE_KEY_B64`.

## 4) Set GitHub repository secrets

- `APT_GPG_PRIVATE_KEY_B64`
- `APT_GPG_KEY_ID`
- `APT_GPG_PASSPHRASE`

## 5) Publish

Tag and push:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The `Publish APT Repository` workflow will build, sign, and publish to `gh-pages`.
