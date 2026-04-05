# Contributing to Gimble

Thanks for your interest in contributing! We welcome improvements, fixes, and new ideas.

Please read our [Code of Conduct](CODE_OF_CONDUCT.md) before participating.

## Getting Started

1. Fork the repo and clone your fork.
2. Create a feature branch: `git checkout -b your-branch-name`.
3. Build locally:

```bash
make build
```

If you don't use `make`, you can build directly:

```bash
go build -o ./gimble ./cmd/gimble
```

## Issues

If you find a bug or want a feature:

- Search [existing issues](https://github.com/Saketspradhan/Gimble-dev/issues) first.
- Include clear repro steps, expected behavior, and your environment.
- Screenshots or logs are extremely helpful.

## Pull Requests

We actively welcome PRs. To keep reviews fast:

- Keep changes scoped and focused.
- Link your PR to a related issue.
- For larger changes, open an issue first to align on approach.

Before submitting, please run:

```bash
make build
```

If you add tests, run them (if applicable):

```bash
go test ./...
```

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
