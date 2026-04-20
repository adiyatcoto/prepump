# Contributing to PrePump Scanner

First off, thank you for considering contributing to PrePump Scanner! It's people like you that make PrePump such a great tool.

## Code of Conduct

This project and everyone participating in it is governed by our Code of Conduct. By participating, you are expected to uphold this code. Please report unacceptable behavior to the project maintainers.

## I Don't Want to Read This Whole Thing, I Just Have a Question

> **Note:** Please don't file an issue to ask a question. You'll get faster results by using the resources below.

- Check the [README.md](../README.md) - your question might already be answered
- Search existing [issues](https://github.com/adiyatcoto/prepump/issues) - someone might have asked the same thing
- Check the [FAQ section](../README.md#troubleshooting) in the README

## What We Are Looking For

We welcome contributions in many forms:

- **Bug reports** - Detailed reports about bugs you've found
- **Feature requests** - Ideas for new features or improvements
- **Documentation** - Improvements to README, comments, or additional guides
- **Code contributions** - Bug fixes, new features, performance improvements
- **Testing** - Help testing new features or verifying bug fixes

## How to Contribute

### Reporting Bugs

Before creating bug reports, please check the existing issues as you might find out that you don't need to create one. When you are creating a bug report, please include as many details as possible:

**Template:**

```markdown
**Describe the bug**
A clear and concise description of what the bug is.

**To Reproduce**
Steps to reproduce the behavior:
1. Run command '...'
2. See error

**Expected behavior**
A clear and concise description of what you expected to happen.

**Screenshots**
If applicable, add screenshots to help explain your problem.

**Environment:**
- OS: [e.g. macOS 14.0, Ubuntu 22.04, Windows 11]
- Go version: [e.g. 1.24.2]
- PrePump version: [e.g. v4.0.0]
- Terminal emulator: [e.g. iTerm2, Windows Terminal]

**Additional context**
Add any other context about the problem here.
```

### Suggesting Features

Feature suggestions are always tracked as [GitHub issues](https://github.com/adiyatcoto/prepump/issues).

**Template:**

```markdown
**Is your feature request related to a problem? Please describe.**
A clear and concise description of what the problem is. Ex. I'm always frustrated when [...]

**Describe the solution you'd like**
A clear and concise description of what you want to happen.

**Describe alternatives you've considered**
A clear and concise description of any alternative solutions or features you've considered.

**Additional context**
Add any other context or screenshots about the feature request here.
```

### Your First Code Contribution

Unsure where to begin contributing? You can start by looking through these `good first issue` and `help wanted` issues:

- **Good first issues** - Issues that should only require a few lines of code
- **Help wanted issues** - Issues that are more involved and need extra help

### Making Changes

1. **Fork** the repository on GitHub
2. **Clone** your fork locally:
   ```bash
   git clone https://github.com/adiyatcoto/prepump.git
   cd prepump
   ```

3. **Create a branch** for your changes:
   ```bash
   git checkout -b feature/your-feature-name
   ```

4. **Make your changes** following our coding standards below

5. **Test your changes**:
   ```bash
   # Run all tests
   make test
   
   # Build the binary
   make build
   
   # Run the application
   ./prepump
   ```

6. **Commit your changes**:
   ```bash
   git add .
   git commit -m "feat: add your feature description"
   ```

7. **Push** to your fork:
   ```bash
   git push origin feature/your-feature-name
   ```

8. **Open a Pull Request** on GitHub

### Commit Message Guidelines

We follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

**Types:**

- `feat`: A new feature
- `fix`: A bug fix
- `docs`: Documentation only changes
- `style`: Changes that don't affect the meaning (white-space, formatting, etc.)
- `refactor`: A code change that neither fixes a bug nor adds a feature
- `perf`: A code change that improves performance
- `test`: Adding missing tests or correcting existing tests
- `chore`: Changes to the build process or auxiliary tools

**Examples:**

```
feat(scanner): add CVD divergence detection
fix(tui): resolve price flash animation flickering
docs: update installation instructions in README
refactor(hmm): optimize matrix calculations for better performance
test(scanner): add unit tests for signal normalization
```

### Coding Standards

#### Go Style

- Follow the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use `gofmt` or `go fmt` to format your code
- Keep functions small and focused (ideally < 50 lines)
- Use meaningful variable and function names
- Add comments for complex logic, but prefer self-documenting code

#### Testing

- Write tests for new features
- Maintain or improve code coverage
- Ensure all existing tests pass:
  ```bash
  go test -cover ./...
  ```

#### Documentation

- Update README.md if you add or change features
- Add godoc comments for exported functions and types
- Keep documentation clear and concise

### Pull Request Process

1. Ensure all tests pass and code is properly formatted
2. Update the README.md with details of changes if needed
3. The PR will be merged once you have the sign-off of at least one maintainer

### Pull Request Template

When you create a PR, please use the following template:

```markdown
## Description

Brief description of the changes in this PR.

## Type of Change

- [ ] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] Documentation update
- [ ] Performance improvement
- [ ] Code refactoring

## Testing

Describe how you tested your changes:

- [ ] Added unit tests
- [ ] Manually tested the feature
- [ ] Verified existing tests still pass

## Checklist

- [ ] My code follows the style guidelines of this project
- [ ] I have performed a self-review of my own code
- [ ] I have commented my code, particularly in hard-to-understand areas
- [ ] I have made corresponding changes to the documentation
- [ ] My changes generate no new warnings
- [ ] I have added tests that prove my fix is effective or that my feature works
- [ ] New and existing unit tests pass locally with my changes

## Screenshots (if applicable)

Add screenshots to help explain your changes.

## Related Issues

Closes #(issue number)
```

## Development Setup

### Prerequisites

- Go 1.24.2 or higher
- Git
- Make (optional but recommended)

### Setup

```bash
# Clone your fork
git clone https://github.com/adiyatcoto/prepump.git
cd prepump

# Download dependencies
go mod download

# Verify build
make build

# Run tests
make test
```

## Architecture Overview

Before contributing, it helps to understand the architecture:

```
cmd/prepump/          # Main entry point, orchestrates everything
internal/scanner/     # Signal calculation engine
internal/tui/         # Bubble Tea terminal UI
internal/pyth/        # Pyth Network SSE client
internal/deepcoin/    # Deepcoin API client
internal/hmm/         # Hidden Markov Model implementation
internal/cache/       # Candle data caching
internal/config/      # Configuration loading
```

### Key Concepts

1. **Scanner Engine**: Calculates 9 weighted signals for each coin
2. **TUI**: Renders the terminal interface using Bubble Tea
3. **Data Streams**: Pyth SSE and Deepcoin provide real-time market data
4. **HMM**: Machine learning model for market regime detection

## Questions?

If you have any questions or need help, feel free to:

- Open an issue on GitHub
- Check existing documentation
- Look at existing code for examples

Thank you for contributing to PrePump Scanner! 🎉
