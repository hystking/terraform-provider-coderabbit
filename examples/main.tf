terraform {
  required_providers {
    coderabbit = {
      source = "registry.terraform.io/hystking/coderabbit"
    }
  }
}

provider "coderabbit" {
  # API key can be set via CODERABBITAI_API_KEY environment variable
  # api_key = "your-api-key"
}

# Assign seats to users using their GitHub username
resource "coderabbit_seats" "user1" {
  github_id = "octocat"
}

resource "coderabbit_seats" "user2" {
  github_id = "defunkt"
}

# Data source to list all seat assignments
data "coderabbit_seats" "all" {}

output "users_with_seats" {
  value = data.coderabbit_seats.all.users_with_seats
}

output "users_without_seats" {
  value = data.coderabbit_seats.all.users_without_seats
}

output "total_seats" {
  value = data.coderabbit_seats.all.total_seats
}

# Output the resolved git_user_id for reference
output "user1_git_user_id" {
  value = coderabbit_seats.user1.git_user_id
}
