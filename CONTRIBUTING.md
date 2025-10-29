# Contributing to Klever Blockchain

Thank you for your interest in contributing to the Klever blockchain! We welcome contributions from the community and are grateful for your support.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [How to Contribute](#how-to-contribute)
- [Development Workflow](#development-workflow)
- [Coding Standards](#coding-standards)
- [Testing Guidelines](#testing-guidelines)
- [Commit Message Guidelines](#commit-message-guidelines)
- [Pull Request Process](#pull-request-process)
- [Issue Reporting](#issue-reporting)
- [Community](#community)

## Code of Conduct

This project and everyone participating in it is governed by our [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code. Please report unacceptable behavior to conduct@klever.org.

## Getting Started

### Prerequisites

Before you begin, ensure you have:

- Go version as specified in `go.mod` or higher
- Git configured with your name and email
- A GitHub account
- Familiarity with blockchain concepts
- Understanding of Go programming

### Setting Up Your Development Environment

1. **Fork the repository** on GitHub

2. **Clone your fork**:
   ```bash
   git clone https://github.com/YOUR-USERNAME/klever-go.git
   cd klever-go
   ```

3. **Add upstream remote**:
   ```bash
   git remote add upstream https://github.com/klever-io/klever-go.git
   ```

4. **Install dependencies**:
   ```bash
   make prepare
   ```

5. **Build the project**:
   ```bash
   make build
   ```

6. **Run tests to verify setup**:
   ```bash
   make tests
   ```

## How to Contribute

There are many ways to contribute to Klever:

### Code Contributions

- **Bug fixes**: Fix issues reported in GitHub Issues
- **New features**: Implement features from the roadmap or propose new ones
- **Performance improvements**: Optimize existing code
- **Refactoring**: Improve code quality and maintainability
- **Test coverage**: Add or improve tests

### Non-Code Contributions

- **Documentation**: Improve README, code comments, or user guides
- **Bug reports**: Report issues with detailed reproduction steps
- **Feature requests**: Suggest new features or improvements
- **Code review**: Review pull requests from other contributors
- **Community support**: Help other users in forums

## Development Workflow

### 1. Find or Create an Issue

- Check [existing issues](https://github.com/klever-io/klever-go/issues) for something to work on
- For new features or significant changes, create an issue first to discuss your approach
- Comment on the issue to let others know you're working on it

### 2. Create a Feature Branch

Always work on a feature branch, never directly on `develop`:

```bash
git checkout develop
git pull upstream develop
git checkout -b feature/your-feature-name
```

Branch naming conventions:
- `feature/feature-name` - New features
- `fix/bug-name` - Bug fixes
- `refactor/component-name` - Code refactoring
- `docs/section-name` - Documentation updates
- `test/component-name` - Test additions or improvements

For Jira-tracked issues, include the ticket number:
```bash
git checkout -b KLC-1234-add-new-consensus-feature
```

### 3. Make Your Changes

- Write clean, readable code following our [Coding Standards](#coding-standards)
- Add tests for new functionality
- Update documentation as needed
- Keep commits focused and atomic

### 4. Test Your Changes

Run the full test suite before submitting:

```bash
# Run all tests
make tests

# Run specific test suites
make tests-unit
make tests-integration
make tests-kvm

# Run specific package tests
go test ./core/process/block/...
```

Ensure your changes don't break existing functionality.

### 5. Commit Your Changes

Follow our [Commit Message Guidelines](#commit-message-guidelines):

```bash
git add .
git commit -m "feat(consensus): add new validator selection algorithm"
```

### 6. Keep Your Branch Updated

Regularly sync with upstream to avoid conflicts. **Always use rebase, not merge:**

```bash
git fetch upstream
git rebase upstream/develop
```

**Important:** We use rebase to maintain a clean, linear history. Do not merge the develop branch into your feature branch.

### 7. Push and Create Pull Request

```bash
git push origin feature/your-feature-name
```

Then create a pull request on GitHub following our [Pull Request Process](#pull-request-process).

## Coding Standards

### Go Style Guide

We follow the [official Go style guide](https://go.dev/doc/effective_go) and common Go idioms:

#### Formatting

- Use `make goimports` to format all Go code and organize imports
- Alternatively, use `gofmt` for basic formatting
- Use tabs for indentation
- Keep line length reasonable (aim for 100-120 characters)
- Use meaningful variable and function names

#### Naming Conventions

```go
// Exported functions and types use PascalCase
func ProcessBlock(block *Block) error { ... }

// Unexported functions and variables use camelCase
func validateTransaction(tx *Transaction) bool { ... }

// Constants use PascalCase or SCREAMING_SNAKE_CASE for clarity
const MaxBlockSize = 1024 * 1024
const DEFAULT_TIMEOUT = 30

// Interfaces are often named with agent nouns (Reader, Writer, Processor, Handler)
// describing what the implementer does
type Reader interface {
    Read(p []byte) (n int, err error)
}

type BlockProcessor interface {
    ProcessBlock(block *Block) error
}
```

#### Code Organization

- Keep files focused on a single responsibility
- Group related functionality in packages
- Use meaningful package names (short, lowercase, no underscores)
- Avoid circular dependencies

#### Error Handling

```go
// Always check and handle errors
result, err := DoSomething()
if err != nil {
    return nil, fmt.Errorf("failed to do something: %w", err)
}

// Use error wrapping for context
if err := validateBlock(block); err != nil {
    return fmt.Errorf("block validation failed at height %d: %w", block.Height, err)
}
```

#### Comments

- Add comments for exported functions, types, and constants
- Explain "why" not "what" in comments
- Use complete sentences with proper punctuation

```go
// ProcessTransaction validates and adds a transaction to the mempool.
// It returns an error if the transaction is invalid or the mempool is full.
func ProcessTransaction(tx *Transaction) error {
    // Verify signature before expensive validation
    if !tx.VerifySignature() {
        return ErrInvalidSignature
    }

    return s.mempool.Add(tx)
}
```

#### Interface Design

- Keep interfaces small and focused
- Define interfaces where they are used, not where they are implemented
- Accept interfaces, return structs

```go
// Good: small, focused interface
type BlockValidator interface {
    ValidateBlock(block *Block) error
}

// Better: define at usage point
type blockProcessor struct {
    validator BlockValidator // interface used here
}

func NewBlockProcessor(v BlockValidator) *blockProcessor {
    return &blockProcessor{validator: v}
}
```

#### Concurrency

- Use channels for communication between goroutines
- Protect shared state with mutexes
- Always clean up goroutines to prevent leaks
- Document concurrency guarantees

```go
// Close channels to signal completion
done := make(chan struct{})
go func() {
    defer close(done)
    // do work
}()
<-done

// Protect shared state
type SafeCounter struct {
    mu    sync.RWMutex
    count int
}

func (c *SafeCounter) Increment() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.count++
}
```

### Blockchain-Specific Guidelines

#### Gas Costs

- Document gas cost rationale
- Use constants for gas values
- Test gas metering thoroughly

#### Consensus Changes

- Consensus changes require extensive testing
- Document protocol version compatibility
- Consider backward compatibility
- Add feature flags in `enableEpochs.yaml`

#### Security Considerations

- Validate all inputs
- Use safe math operations (check for overflow/underflow)
- Sanitize user data before processing
- Review cryptographic operations carefully
- Consider DoS attack vectors

```go
// Always validate input bounds
func ProcessAmount(amount uint64) error {
    if amount > MaxTransactionAmount {
        return ErrAmountTooLarge
    }
    return nil
}

// Use safe math
if amount1 > math.MaxUint64 - amount2 {
    return ErrOverflow
}
result := amount1 + amount2
```

## Testing Guidelines

### Test Coverage

- Aim for 80%+ test coverage on new code
- Write tests before fixing bugs (test-driven development)
- Test both success and failure cases
- Test edge cases and boundary conditions

### Test Structure

```go
func TestProcessTransaction(t *testing.T) {
    // Arrange: set up test data
    tx := &Transaction{
        From:   "address1",
        To:     "address2",
        Amount: 100,
    }

    processor := NewProcessor()

    // Act: execute the test
    err := processor.ProcessTransaction(tx)

    // Assert: verify results
    if err != nil {
        t.Errorf("expected no error, got %v", err)
    }
}
```

### Table-Driven Tests

Use table-driven tests for multiple scenarios:

```go
func TestValidateAmount(t *testing.T) {
    tests := []struct {
        name    string
        amount  uint64
        wantErr bool
    }{
        {"valid amount", 100, false},
        {"zero amount", 0, true},
        {"max amount", math.MaxUint64, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateAmount(tt.amount)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateAmount() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Test Types

#### Unit Tests

- Test individual functions and methods
- Mock external dependencies
- Fast execution (< 1 second per test)

#### Integration Tests

- Test component interactions
- Use real dependencies where practical
- Place in `integrationTest/` directory

#### KVM Tests

- Test smart contract execution
- Verify gas metering
- Test security sandboxing
- Use `kvm/test/` directory

### Mocking

- Use interfaces for dependencies
- Create mocks in `common/mock/` or test files
- Keep mocks simple and focused

```go
type MockValidator struct {
    ValidateFunc func(*Block) error
}

func (m *MockValidator) Validate(block *Block) error {
    if m.ValidateFunc != nil {
        return m.ValidateFunc(block)
    }
    return nil
}
```

## Commit Message Guidelines

We follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

### Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Type

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, no logic change)
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `test`: Adding or updating tests
- `chore`: Maintenance tasks, dependency updates

### Scope

The scope should specify the area of change:

- `consensus`: Consensus mechanism
- `kvm`: Virtual machine
- `network`: Networking and P2P
- `api`: REST API
- `storage`: Storage layer
- `kapp`: KApp system
- `crypto`: Cryptographic operations

### Subject

- Use imperative mood: "add feature" not "added feature"
- Don't capitalize first letter
- No period at the end
- Keep under 50 characters

### Body (Optional)

- Explain what and why, not how
- Wrap at 72 characters
- Separate from subject with blank line

### Footer (Optional)

- Reference issues: `Fixes #123`, `Closes #456`
- Note breaking changes: `BREAKING CHANGE: description`

### Examples

```
feat(consensus): add validator rotation mechanism

Implement round-robin validator rotation to improve
decentralization and prevent validator monopolization.

Fixes #1234
```

```
fix(kvm): prevent gas metering overflow

Add overflow checks in gas calculation to prevent
potential DoS attacks through gas exhaustion.

Fixes #5678
```

```
docs(readme): update installation instructions

Add prerequisites section and clarify build steps
for new contributors.
```

### Jira Integration

For Jira-tracked issues, include the ticket in the commit:

```
feat(consensus): add new validator selection algorithm

[KLC-1234] Implement weighted random selection for validators
based on stake amount and performance history.
```

## Pull Request Process

### Before Submitting

1. ✅ All tests pass (`make tests`)
2. ✅ Code follows style guidelines (`make goimports`)
3. ✅ Documentation is updated
4. ✅ Commit messages follow guidelines
5. ✅ Branch is up to date with `develop`
6. ✅ No unnecessary files included

### PR Title

For Jira-tracked issues, include the ticket number:

```
[KLC-1234] Add new validator selection algorithm
```

Otherwise, use conventional commit format:

```
feat(consensus): add new validator selection algorithm
```

### PR Description Template

See `.github/PULL_REQUEST_TEMPLATE.md` for the full template that auto-populates when creating a PR.

Key sections to include:
- **Summary** - Brief description of what this PR does
- **Problem** - What issue does this solve?
- **Solution** - How does this PR solve the problem?
- **Key Changes** - List main changes (New/Updated/Removed)
- **Testing** - How was this tested?
- **Configuration Changes** - Document any config changes required
- **Breaking Changes** - List any breaking changes
- **Related Issues** - Use `Fixes #123` or `Related to #456`
- **Checklist** - Verify all requirements are met

### Review Process

1. **Automated checks**: CI/CD pipeline runs tests and linters
2. **Code review**: At least one maintainer reviews the code
3. **Feedback**: Address review comments and update the PR
4. **Approval**: Once approved, your PR will be merged
5. **Merge**: Maintainers will merge using squash or rebase

### After Merge

- Your changes will be included in the next release
- Delete your feature branch
- Update your local repository

```bash
git checkout develop
git pull upstream develop
git branch -d feature/your-feature-name
```

## Issue Reporting

### Bug Reports

When reporting bugs, include:

1. **Description**: Clear description of the bug
2. **Steps to reproduce**: Detailed steps to recreate the issue
3. **Expected behavior**: What should happen
4. **Actual behavior**: What actually happens
5. **Environment**: OS, Go version, node version
6. **Logs**: Relevant error messages or logs
7. **Screenshots**: If applicable

### Feature Requests

When requesting features, include:

1. **Use case**: Why is this feature needed?
2. **Proposed solution**: How should it work?
3. **Alternatives**: Other approaches considered
4. **Impact**: Who benefits from this feature?

### Issue Labels

- `bug`: Something isn't working
- `enhancement`: New feature or improvement
- `documentation`: Documentation updates
- `good first issue`: Good for newcomers
- `help wanted`: Extra attention needed
- `question`: Further information requested
- `wontfix`: This will not be worked on

## Community

### Communication Channels

- **GitHub Issues**: Bug reports and feature requests
- **GitHub Discussions**: Questions and general discussion
- **Forum**: Long-form discussions and support
- **Twitter**: Announcements and updates

### Getting Help

- Check the [documentation](https://docs.klever.org)
- Search [existing issues](https://github.com/klever-io/klever-go/issues)
- Post in the [forum](https://forum.klever.org)

### Recognition

We value all contributions! Contributors are:

- Listed in release notes
- Acknowledged in the project
- Given credit in relevant documentation
- Invited to contributor events

## License

By contributing to Klever, you agree that your contributions will be licensed under the [Apache License 2.0](LICENSE).

---

Thank you for contributing to Klever! Your efforts help make the blockchain ecosystem better for everyone.
