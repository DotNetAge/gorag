# Contributing to GoRAG

Thank you for your interest in contributing to GoRAG!

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/your-username/gorag.git`
3. Create a feature branch: `git checkout -b feature/my-feature`
4. Make your changes
5. Run tests: `go test ./...`
6. Submit a pull request

## Development Setup

```bash
# Install dependencies
go mod download

# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run linter
golangci-lint run
```

## Code Style

- Follow standard Go conventions
- Use `gofmt` for formatting
- Write tests for new features
- Document exported functions and types

## Commit Messages

Use clear and descriptive commit messages:

```
feat: add PDF parser support
fix: resolve memory leak in vector store
docs: update README with new examples
```

## Pull Request Process

1. Ensure all tests pass
2. Update documentation if needed
3. Add tests for new features
4. Keep PRs focused and small

## Questions?

Feel free to open an issue for any questions or discussions.
