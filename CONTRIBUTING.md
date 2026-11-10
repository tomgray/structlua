# Contributing Guide

Thank you for your interest in contributing to `structlua`! We welcome contributions from everyone.

## Code of Conduct

Please be respectful, constructive, and considerate of others when participating in this project's discussions and development.

## Reporting Bugs

Before submitting a new issue, please search existing issues to see if the bug has already been reported.

When filing a bug report, please create an issue that includes:
- **A clear summary** of the problem.
- **Expected behavior**: What you expected to happen.
- **Observed behavior**: What actually happened.
- **Steps to reproduce**: Minimal step-by-step instructions or test cases reproducing the issue.
- **Supporting details**:
  - Relevant logs, stack traces, or Lua script snippets.
  - Environment and system details (OS, Go version, Lua backend implementation: `go-lua` or `gopher-lua`).

## Development Environment Setup

This project uses the Nix build system with Nix Flakes (`flake.nix`) and a Go multi-module workspace (`go.work`).

- **Enter development environment**:
  ```bash
  nix develop
  ```
- **Run tests across all modules**:
  ```bash
  nix run .#test
  # or:
  nix develop -c go test -v ./... ./golua/... ./gopherlua/...
  ```
- **Run formatting**:
  ```bash
  nix fmt
  ```

## Submitting Improvements (Pull Requests)

We gladly accept improvements, bug fixes, and feature enhancements via Pull Requests.

To submit a pull request:
1. **Fork and branch**: Fork the repository and create a descriptive feature branch from `master`.
2. **Follow project guidelines**:
   - Keep changes simple and focused: avoid speculative generalizations or unnecessary abstractions.
   - Follow Test-Driven Development (TDD): ensure your changes are covered by tests (preference for table-driven tests in Go).
   - Format all code with `nix fmt` before committing.
3. **Commit clearly**: Write meaningful commit messages explaining *why* the change was made and reference any relevant issue numbers (e.g. `Resolves #123`).
4. **Open a PR**:
   - Provide a clear summary of the changes and problem solved.
   - Verify that all tests pass.
