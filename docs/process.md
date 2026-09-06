## Stage cycle

Each stage in [01-tz.md](01-tz.md) goes through the same cycle:

1. Write tests (spec / contracts)
2. Review tests
3. Implement the feature
4. All tests green
5. Acceptance

## Definition of Done (DoD)

A stage is considered done when:

- [ ] Feature is implemented following rules;
- [ ] Tests approved;

## Rules

1. **Tests before code.** The essence of the tests is reviewed by the developer before they are written. Tests should not test every function there is, they should test critical parts of code in the first place.
2. **One stage — one vertical slice.** Implement features in the vertical slices. The scenario works from the UI to the backend (or from the adapter to the UI).
3. **Review is blocking.** Without your approval of the tests the next step does not start.
4. **CI.** All tests run on every commit to `main` / every PR.
5. **Changing the contract or architecture requires an ADR** (see `docs/adr/README.md`).
6. **Exceptions.** If implementation uncovers a nuance that was not in the tests — the tests are extended, the diff is reviewed, then the fix.
7. **Commit format.** Only [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `chore:`, `docs:` etc.). Violations are blocked by the local `commit-msg` hook (husky + commitlint, at the repo root).

## Tools

| Layer         | Tool                       | Command                                     |
| ------------- | -------------------------- | ------------------------------------------- |
| Go backend    | `go test` (stdlib testing) | `go test ./...`                             |
| Go static     | `go vet` / `go fmt`        | `go vet ./...`, `go fmt ./...`              |
| UI components | vitest + vue-test-utils    | `pnpm test`                                 |
| UI lint       | eslint + prettier          | `pnpm lint`, `pnpm format:check`            |
| E2E           | Playwright                 | `pnpm exec playwright test`                 |
| Git commits   | husky + commitlint         | `git commit` (`commit-msg` hook, repo root) |
| CI            | GitHub Actions             | `.github/workflows/test.yml`                |
