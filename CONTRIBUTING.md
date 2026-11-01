# Contributing to capimgr

Thank you for considering a contribution. Issues and pull requests are welcome.

## Before you start

- Search existing issues and pull requests to avoid duplicating work.
- For a large change, open an issue first to discuss the approach and scope.
- Keep changes focused, preserve existing behavior, and update relevant documentation.
- Do not include credentials, kubeconfigs, cluster dumps, or other sensitive data.

## Development and checks

The project has a Go backend and a React/Vite frontend. Run relevant checks before
opening a pull request:

```sh
cd backend
go test ./...
go vet ./...
```

```sh
cd frontend
npm ci
node --test src/locale.test.js src/providerLogos.test.js
npm run build
```

Changes to Kubernetes deployment resources should be checked with the applicable
Helm or Kubernetes tooling when available. Do not run destructive operations
against a shared or production cluster.

## Pull requests

- Explain the problem and the solution, including any behavior or API changes.
- Link related issues and include screenshots for visible UI changes when useful.
- Add or update tests for changed behavior.
- Keep generated files, unrelated formatting changes, and secrets out of the diff.
- Call out checks that could not be run and why.

By submitting a contribution for inclusion, you agree that it is submitted under
the project's Apache License 2.0, as described in section 5 of that license.
