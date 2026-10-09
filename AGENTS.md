# AGENTS.md

## Communication

- Keep responses short. Report outcomes and open questions, not a recap of steps.
- Be honest. State plainly when I'm wrong, when something is a bad idea, or when you don't know.
- No flattery, praise, apologies, or enthusiasm. Don't agree just to be agreeable.
- If something needs my decision, ask. Otherwise, proceed.

## Design Process

Applies to anything architectural: a new package, component, exported interface, dependency, or change to boundaries. Small local changes don't need it.

1. Ask how I would implement it. Don't share your idea yet.
2. Evaluate my answer. If it has a flaw, say so. If it hinges on a concept I seem to be missing, name the concept and link a good reference (official docs, specs, papers) instead of just giving the conclusion.
3. Then share your own approach, with trade-offs.
4. Iterate until we agree. Only then write code.

- If the purpose is unclear, ask sharp questions (most important first, max 3) before designing.
- If the work is big, split it into phases. Plan one, code it, get my approval, then continue.

## Scope

- Don't write code you weren't told to write. No extra features, refactors, or cleanups. Mention them briefly and wait.
- Exception: tests and docs for the code you write are always in scope.
- When unsure, ask instead of guessing.

## Go

- Idiomatic Go. Standard library first. Ask before adding any dependency.
- Wrap errors with `fmt.Errorf("...: %w", err)`. Handle each error once: don't log and return it.
- Accept interfaces, return structs. Define small interfaces where they are consumed.
- No global mutable state, no `init()` side effects, no `util`/`common`/`helpers` packages.
- Every goroutine has an owner and stops via context cancellation.
- No magic numbers or strings. Prefer plain code over reflection, heavy generics, or codegen unless there is a measured performance reason (document it).
- Short names in small scopes, descriptive names in wide ones.

## Comments

- Only when needed, and only for *why*. Link a reference for well-known algorithms.
- Explanations belong in `docs/`, not in code.

## Testing

- Table-driven tests with subtests. Test behavior, not implementation.
- Never delete, skip, or loosen a test to make it pass. If a test seems wrong, ask me.

## Definition of Done

- `gofmt -l .` prints nothing
- `go vet ./...` is clean
- `go test ./...` passes
- `docs/` and `README.md` updated if affected

## Git

- Conventional Commits: `type(scope): description`. Mark breaking changes with `!`.
- Small, focused commits.
- Never commit or push unless I tell you to.
