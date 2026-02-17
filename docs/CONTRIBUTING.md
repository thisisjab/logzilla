# Contributing to Logzilla

Thank you for your interest in contributing! To maintain high code quality and security, we ask that all contributors follow the guidelines below.

# Getting Started

1. Fork the Repository
Begin by forking the repository to your own GitHub account. Clone your fork locally:

```Bash
git clone https://github.com/thisisjab/logzilla.git
cd logzilla
```

2. Environment Setup

This project utilizes pre-commit to ensure code consistency and linting standards. You must install the hooks before making any changes:

```Bash
# Ensure pre-commit is installed on your system
pip install pre-commit

# Install the git hooks
pre-commit install
```

3. Implement Your Changes

Create a new branch for your feature or bug fix:

```Bash
git checkout -b feat/your-feature-name
```

4. Commit Requirements

We enforce strict standards for all commits:

Pre-commit Hooks: All files must pass the automated checks (Linting, Formatting, etc.) configured in this repository. If a hook fails, the commit will be rejected until the issues are resolved.

Conventional Commits: We follow the Conventional Commits specification (e.g., feat: add logging, fix: handle null pointer).

Verified Commits: All commits must be GPG signed and verified. Unverified commits will not be merged into the main branch.

To sign your commit, use: git commit -S -m "feat: your message"

# Submitting Your Contribution

5. Push and Create a Pull Request

Once your changes are committed and verified:

Push the branch to your fork: git push origin feat/your-feature-name.

Navigate to the original repository on GitHub.

Open a Pull Request (PR) with a clear description of the changes and the problem they solve.

6. Code Review

An automated CI suite will run against your PR. Once passing, a maintainer will review your code. Please stay active in the PR thread to address any requested changes.
