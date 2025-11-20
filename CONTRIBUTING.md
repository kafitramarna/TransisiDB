# Contributing to TransisiDB

Thank you for your interest in TransisiDB! This is a capstone/portfolio project, but feedback and suggestions are always welcome.

## How to Contribute

### Reporting Issues

If you find a bug or have a suggestion:

1. Check if the issue already exists
2. Create a new issue with:
   - Clear description
   - Steps to reproduce (for bugs)
   - Expected vs actual behavior
   - Environment details (OS, Go version, etc.)

### Suggesting Enhancements

Enhancement suggestions are welcome! Please:

1. Describe the feature clearly
2. Explain the use case
3. Consider backward compatibility

### Code Contributions

While this is primarily a portfolio project, quality contributions are appreciated:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes with clear messages
4. Add/update tests as needed
5. Update documentation
6. Submit a Pull Request

### Code Standards

- Follow Go conventions and idioms
- Run `go fmt` before committing
- Ensure all tests pass (`go test ./...`)
- Add comments for complex logic
- Update relevant documentation

## Development Setup

```bash
# Clone your fork
git clone https://github.com/kafitramarna/TransisiDB.git
cd TransisiDB

# Install dependencies
go mod download

# Run tests
go test ./...

# Build
go build ./cmd/...
```

## Testing

- Write unit tests for new features
- Maintain test coverage
- Include integration tests where applicable
- Document test scenarios

## Documentation

When contributing, please update:

- Code comments
- README.md (if feature changes UX)
- API.md (if adding/changing endpoints)
- Inline documentation

## Questions?

Feel free to open an issue for questions or clarifications.

---

**Note:** As this is a portfolio project, response times may vary. Your patience is appreciated!
