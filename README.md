# Terraform Provider for CodeRabbit

A Terraform provider for managing CodeRabbit seat assignments.

## Features

- **coderabbit_seats resource**: Assign/unassign seats to GitHub users
- **coderabbit_seats data source**: Retrieve current seat assignment status

## Installation

### Local Build

```bash
# Install dependencies
make deps

# Build and install locally
make install
```

### Terraform Registry (after publication)

```hcl
terraform {
  required_providers {
    coderabbit = {
      source  = "hystking/coderabbit"
      version = "~> 0.1.0"
    }
  }
}
```

## Usage

### Provider Configuration

```hcl
provider "coderabbit" {
  # API key can also be set via CODERABBITAI_API_KEY environment variable
  api_key = "your-api-key"

  # Optional: Custom API endpoint (default: https://api.coderabbit.ai)
  # base_url = "https://api.coderabbit.ai"
}
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `CODERABBITAI_API_KEY` | CodeRabbit API authentication key |
| `CODERABBIT_BASE_URL` | API base URL (optional) |

### Assigning Seats

Specify a GitHub username to assign a seat. The provider automatically resolves the username to a numeric user ID via the GitHub API.

```hcl
resource "coderabbit_seats" "developer1" {
  github_id = "octocat"
}

resource "coderabbit_seats" "developer2" {
  github_id = "defunkt"
}
```

#### Attributes

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `github_id` | string | Yes | GitHub username (e.g., "octocat") |
| `git_user_id` | string | - | Resolved numeric GitHub user ID (computed) |
| `id` | string | - | Resource ID (computed) |

### Retrieving Seat Information

```hcl
data "coderabbit_seats" "all" {}

output "users_with_seats" {
  value = data.coderabbit_seats.all.users_with_seats
}

output "users_without_seats" {
  value = data.coderabbit_seats.all.users_without_seats
}
```

#### Attributes

| Attribute | Type | Description |
|-----------|------|-------------|
| `users_with_seats` | list(string) | List of user IDs with assigned seats |
| `users_without_seats` | list(string) | List of user IDs without assigned seats |

## Complete Example

```hcl
terraform {
  required_providers {
    coderabbit = {
      source = "hystking/coderabbit"
    }
  }
}

provider "coderabbit" {}

# Assign seats to team members
resource "coderabbit_seats" "team" {
  for_each  = toset(["alice", "bob", "charlie"])
  github_id = each.value
}

# Check current seat status
data "coderabbit_seats" "current" {}

output "assigned_users" {
  value = data.coderabbit_seats.current.users_with_seats
}
```

## Development

### Requirements

- Go 1.21+
- Terraform 1.0+

### Commands

```bash
# Build
make build

# Local install
make install

# Test
make test

# Format
make fmt

# Static analysis
make vet

# Clean up
make clean
```

## Reference Documentation

- [CodeRabbit API Documentation](https://api.coderabbit.ai/v1/docs/)

## License

MIT License
