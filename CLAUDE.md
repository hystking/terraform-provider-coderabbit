# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Terraform provider for managing CodeRabbit seat assignments. Built with Terraform Plugin Framework.

## Common Commands

```bash
make deps      # Install dependencies (go mod tidy)
make build     # Build the provider binary
make install   # Build and install to local Terraform plugins directory
make test      # Run all tests
make fmt       # Format Go code
make vet       # Run static analysis
make clean     # Remove binary and installed plugins
```

After `make install`, use `terraform init` in a directory with a `.tf` file to test locally.

## Architecture

```
main.go                           # Plugin entry point, starts provider server
internal/
  provider/
    provider.go                   # Provider definition, configuration, schema
  client/
    client.go                     # CodeRabbit API client (HTTP calls to api.coderabbit.ai)
  resources/
    seats_resource.go             # coderabbit_seats resource (CRUD operations)
    seats_data_source.go          # coderabbit_seats data source (read-only)
```

### Key Patterns

- **Provider Configuration**: API key from `CODERABBITAI_API_KEY` env var or `api_key` attribute
- **GitHub ID Resolution**: The `coderabbit_seats` resource accepts `github_id` (username) and resolves it to numeric `git_user_id` via GitHub API
- **Idempotency**: Create/Delete operations check current state before calling API to avoid duplicate operations
- **Import Support**: Resources can be imported using `terraform import coderabbit_seats.name github_username`

### API Endpoints Used

- `GET /v1/seats/` - List all users with seat status
- `POST /v1/seats/assign` - Assign seat to user
- `POST /v1/seats/unassign` - Unassign seat from user

API docs: https://api.coderabbit.ai/v1/docs/
