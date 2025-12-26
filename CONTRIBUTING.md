# Contributing to ProxyPilot

Thank you for your interest in contributing to ProxyPilot! This guide will help you get started.

## Development Environment Setup

1. **Install Go 1.24+**
   - Download from [golang.org](https://golang.org/dl/)
   - Verify installation: `go version`

2. **Clone the repository**
   ```bash
   git clone https://github.com/your-org/ProxyPilot.git
   cd ProxyPilot
   ```

3. **Install dependencies**
   ```bash
   go mod download
   ```

## Running Tests

Run all tests:
```bash
go test ./...
```

Run tests with coverage:
```bash
go test -cover ./...
```

Run tests for a specific package:
```bash
go test ./pkg/proxy/...
```

## Code Style Guidelines

We follow standard Go conventions:

- **Format code** before committing:
  ```bash
  go fmt ./...
  ```

- **Run static analysis**:
  ```bash
  go vet ./...
  ```

- Follow [Effective Go](https://golang.org/doc/effective_go) guidelines
- Keep functions focused and reasonably sized
- Add comments for exported functions and types
- Use meaningful variable and function names

## Pull Request Process

1. Fork the repository and create a feature branch from `main`
2. Make your changes with clear, atomic commits
3. Ensure all tests pass and add tests for new functionality
4. Run `go fmt` and `go vet` before submitting
5. Open a pull request with a clear description of the changes
6. Link any related issues in the PR description
7. Address review feedback promptly

## Issue Reporting Guidelines

When reporting issues, please include:

- **Bug reports**: Steps to reproduce, expected behavior, actual behavior, Go version, and OS
- **Feature requests**: Clear description of the proposed feature and its use case
- **Questions**: Check existing issues and documentation first

Use descriptive titles and provide as much context as possible.

## Third-Party Provider Integrations

We welcome contributions that add integrations with third-party providers! If you're adding a new provider integration:

- Follow the existing integration patterns in the codebase
- Include documentation for configuration options
- Add appropriate tests
- Update the README if necessary

## Questions?

Feel free to open an issue for any questions about contributing.
