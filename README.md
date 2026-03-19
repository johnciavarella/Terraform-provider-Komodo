# Terraform Provider for Komodo

A Terraform provider for managing infrastructure on the [Komodo](https://komo.do) build and deployment system using the [Komodo Core API](https://docs.rs/komodo_client/latest/komodo_client/api/index.html).

## Table of Contents

- [Requirements](#requirements)
- [Installation](#installation)
- [Authentication](#authentication)
- [Provider Configuration](#provider-configuration)
- [server\_id — Local vs Remote Servers](#server_id--local-vs-remote-servers)
- [Tags](#tags)
- [Resources](#resources)
  - [komodo_server](#komodo_server)
  - [komodo_stack](#komodo_stack)
  - [komodo_deployment](#komodo_deployment)
  - [komodo_build](#komodo_build)
  - [komodo_builder](#komodo_builder)
  - [komodo_repo](#komodo_repo)
  - [komodo_tag](#komodo_tag)
  - [komodo_user](#komodo_user)
  - [komodo_api_key](#komodo_api_key)
  - [komodo_git_provider_account](#komodo_git_provider_account)
- [Data Sources](#data-sources)
- [Examples](#examples)
- [Development](#development)

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.22 (to build the provider)
- A running Komodo Core instance with API credentials

## Installation

### Option 1: Install from Source

```bash
git clone https://github.com/johnciavarella/terraform-provider-komodo
cd terraform-provider-komodo
make install
```

### Option 2: Use Pre-built Binary

Download the appropriate binary for your platform from the releases page and place it in your Terraform plugins directory:

```bash
# Example for macOS ARM64
mkdir -p ~/.terraform.d/plugins/registry.terraform.io/moghtech/komodo/0.1.0/darwin_arm64
cp terraform-provider-komodo ~/.terraform.d/plugins/registry.terraform.io/moghtech/komodo/0.1.0/darwin_arm64/
```

## Authentication

The provider authenticates with the Komodo Core API using an API key and secret. These can be obtained from the Komodo UI Settings page.

### Provider Configuration

```hcl
terraform {
  required_providers {
    komodo = {
      source  = "registry.terraform.io/moghtech/komodo"
      version = "0.1.0"
    }
  }
}

provider "komodo" {
  base_url   = "https://komodo.example.com"
  api_key    = var.komodo_api_key
  api_secret = var.komodo_api_secret
}
```

### Environment Variables

| Provider Attribute | Environment Variable  | Description              |
| ------------------ | --------------------- | ------------------------ |
| `base_url`        | `KOMODO_ADDRESS`      | Komodo Core API URL      |
| `api_key`         | `KOMODO_API_KEY`      | API key for auth         |
| `api_secret`      | `KOMODO_API_SECRET`   | API secret for auth      |

```bash
export KOMODO_ADDRESS="https://komodo.example.com"
export KOMODO_API_KEY="K-your-api-key"
export KOMODO_API_SECRET="S-your-api-secret"
```

## Tags

The `tags` attribute is available on `komodo_server`, `komodo_stack`, `komodo_deployment`, `komodo_build`, `komodo_builder`, and `komodo_repo`. Tags are Komodo label objects (managed with `komodo_tag`) and let you group and filter resources in the Komodo UI.

```hcl
resource "komodo_tag" "env_prod" {
  name = "production"
}

resource "komodo_tag" "team_backend" {
  name = "backend"
}

resource "komodo_stack" "web" {
  name = "web-app"
  tags = [komodo_tag.env_prod.name, komodo_tag.team_backend.name]
  # ...
}

resource "komodo_deployment" "api" {
  name  = "api-service"
  image = "nginx:latest"
  tags  = [komodo_tag.env_prod.name]
  # ...
}
```

**Behaviour:**

- Omitting `tags` (or leaving it `null`) means Terraform does not manage tags on that resource — existing tags set in the UI are preserved.
- Setting `tags = ["production"]` sets exactly those tags, replacing any previously set tags.
- Setting `tags = []` clears all tags from the resource.
- Tags are applied via the `UpdateResourceMeta` API call after each create or update.

## server\_id — Local vs Remote Servers

Most resources (`komodo_stack`, `komodo_deployment`, `komodo_repo`, `komodo_builder`) have an optional `server_id` field that controls which server the resource runs on.

### Default: Local server

If `server_id` is omitted, Komodo uses the built-in **Local** server (the machine running Komodo Core itself):

```hcl
resource "komodo_stack" "my_app" {
  name = "my-app"
  # server_id not set → runs on Local
  file_contents = "..."
}
```

### Targeting a specific server

To deploy to a named server, manage it as a `komodo_server` resource and reference its `.id`:

```hcl
resource "komodo_server" "web" {
  name    = "web-01"
  address = "http://10.0.1.10:8120"
}

resource "komodo_stack" "my_app" {
  name      = "my-app"
  server_id = komodo_server.web.id   # resolves to the canonical server ID
  ...
}

resource "komodo_deployment" "api" {
  name      = "api"
  server_id = komodo_server.web.id
  image     = "nginx:latest"
  ...
}
```

Using `.id` creates an implicit dependency — Terraform will create the server before the stack or deployment, and destroy them in the correct order.

### Server managed outside this config

If the server already exists in Komodo (e.g., managed by a different Terraform config or created manually in the UI), use a data source to look it up by name:

```hcl
data "komodo_server" "web" {
  id = "web-01"   # accepts the server's name or its canonical ID
}

resource "komodo_stack" "my_app" {
  name      = "my-app"
  server_id = data.komodo_server.web.id
  ...
}
```

> **Why not just write `server_id = "web-01"`?**
> The Komodo API requires a canonical server ID (a MongoDB ObjectID like `69bb7c183e035f0f3581f471`), not a display name. The provider resolves names to IDs internally, but Terraform's plan validation requires the plan value to match what the API will return. Using `.id` from either a resource or data source ensures the canonical ID is known at plan time, giving you clean plans with no surprises.

## Resources

### komodo_server

Manages a Komodo server (a machine running Komodo Periphery).

```hcl
resource "komodo_server" "prod" {
  name    = "production-01"
  address = "http://10.0.1.100:8120"
  region  = "us-east-1"
  enabled = true

  stats_monitoring        = true
  auto_prune              = true
  send_unreachable_alerts = true
  send_cpu_alerts         = true
  send_mem_alerts         = true
  send_disk_alerts        = true

  cpu_warning   = 80
  cpu_critical  = 95
  mem_warning   = 75
  mem_critical  = 95
  disk_warning  = 75
  disk_critical = 95
}
```

#### Attributes

| Attribute | Type | Required | Default | Description |
| --------- | ---- | -------- | ------- | ----------- |
| `name` | `string` | Yes | - | Display name of the server |
| `address` | `string` | No | `"http://localhost:8120"` | HTTP address of the Periphery client |
| `external_address` | `string` | No | `""` | External address used for container links |
| `region` | `string` | No | `""` | Region label for the server |
| `enabled` | `bool` | No | `true` | Whether the server is enabled |
| `passkey` | `string` | No | `""` | Passkey for authenticating with Periphery |
| `stats_monitoring` | `bool` | No | `true` | Whether to monitor server stats beyond health check |
| `auto_prune` | `bool` | No | `true` | Whether to trigger docker image prune every 24 hours |
| `send_unreachable_alerts` | `bool` | No | `false` | Send alerts about server reachability |
| `send_cpu_alerts` | `bool` | No | `false` | Send alerts about CPU status |
| `send_mem_alerts` | `bool` | No | `false` | Send alerts about memory status |
| `send_disk_alerts` | `bool` | No | `false` | Send alerts about disk status |
| `cpu_warning` | `number` | No | `90` | CPU warning threshold (%) |
| `cpu_critical` | `number` | No | `99` | CPU critical threshold (%) |
| `mem_warning` | `number` | No | `75` | Memory warning threshold (%) |
| `mem_critical` | `number` | No | `95` | Memory critical threshold (%) |
| `disk_warning` | `number` | No | `75` | Disk warning threshold (%) |
| `disk_critical` | `number` | No | `95` | Disk critical threshold (%) |
| `tags` | `list(string)` | No | `null` | Tag names to apply. `null` = unmanaged; `[]` = clear all tags. |

### komodo_stack

Manages a Komodo stack (Docker Compose stack). Supports three compose source modes: **git repo**, **UI-defined** (inline), and **files on host**.

> **server_id**: Omit to use the Local server. Set to `komodo_server.<name>.id` to target a specific server. See [server\_id — Local vs Remote Servers](#server_id--local-vs-remote-servers).

#### Example: Git-based stack

```hcl
resource "komodo_stack" "web_app" {
  name      = "web-application"
  server_id = komodo_server.prod.id   # omit to use Local

  # Git source
  git_provider  = "github.com"
  git_account   = "myorg"
  repo          = "myorg/web-app"
  branch        = "main"
  file_paths    = ["docker-compose.yml"]

  auto_pull   = true
  send_alerts = true
  environment = "MY_VAR=value\nOTHER_VAR=other"

  deployed = true   # git pull + compose up on create/update
  started  = true   # compose up if deployed is false; compose down if false
}
```

#### Example: UI-defined inline stack (no git repo)

```hcl
resource "komodo_stack" "hello_world" {
  name = "hello-world"
  # server_id omitted → runs on Local

  file_contents = <<-EOT
    services:
      hello:
        image: hello-world:latest
        environment:
          - GREETING=$${GREETING}
  EOT

  environment = "GREETING=Hello"
  deployed    = true
  started     = true
}
```

#### Example: Files on host

```hcl
resource "komodo_stack" "on_host" {
  name          = "on-host-stack"
  server_id     = komodo_server.prod.id
  files_on_host = true
  run_directory = "/opt/my-app"
  file_paths    = ["docker-compose.yml"]

  deployed = true
  started  = true
}
```

#### Attributes

| Attribute | Type | Required | Default | Description |
| --------- | ---- | -------- | ------- | ----------- |
| `name` | `string` | Yes | - | Display name of the stack |
| `server_id` | `string` | No | `"Local"` | Server to deploy on. Use `komodo_server.<name>.id`. Omit for Local. |
| `project_name` | `string` | No | `""` | Custom project name for `docker compose -p` |
| `auto_pull` | `bool` | No | `false` | Run `compose pull` before redeploying |
| `run_build` | `bool` | No | `false` | Run `docker compose build` before deploy |
| `auto_update` | `bool` | No | `false` | Redeploy automatically when newer images are found |
| `destroy_before_deploy` | `bool` | No | `false` | Run `compose down` before `compose up` |
| `git_provider` | `string` | No | `"github.com"` | Git provider domain |
| `git_https` | `bool` | No | `true` | Use HTTPS to clone the repo |
| `git_account` | `string` | No | `""` | Git account for private repos |
| `repo` | `string` | No | `""` | Repository in `namespace/repo_name` format |
| `branch` | `string` | No | `"main"` | Branch to deploy |
| `commit` | `string` | No | `""` | Pin to a specific commit hash |
| `file_paths` | `list(string)` | No | `null` | Paths to compose files |
| `files_on_host` | `bool` | No | `false` | Source compose files from the server's filesystem |
| `file_contents` | `string` | No | `""` | Inline compose YAML (alternative to git/host) |
| `run_directory` | `string` | No | `""` | Directory to `cd` into before `docker compose` |
| `env_file_path` | `string` | No | `".env"` | Env file written before `compose up` |
| `environment` | `string` | No | `""` | Newline-separated `KEY=VALUE` pairs written to the env file |
| `additional_env_files` | `list(string)` | No | `null` | Additional env files via `--env-file` |
| `webhook_enabled` | `bool` | No | `false` | Allow incoming webhooks to trigger actions |
| `webhook_secret` | `string` | No | `""` | Custom webhook secret (sensitive) |
| `webhook_force_deploy` | `bool` | No | `false` | Always deploy on webhook without diffing |
| `pre_deploy_command` | `string` | No | `""` | Shell command to run before deploy |
| `pre_deploy_path` | `string` | No | `""` | Working directory for pre-deploy command |
| `post_deploy_command` | `string` | No | `""` | Shell command to run after deploy |
| `post_deploy_path` | `string` | No | `""` | Working directory for post-deploy command |
| `extra_args` | `list(string)` | No | `null` | Extra arguments after `docker compose up -d` |
| `build_extra_args` | `list(string)` | No | `null` | Extra arguments after `docker compose build` |
| `ignore_services` | `list(string)` | No | `null` | Services excluded from stack health checks |
| `send_alerts` | `bool` | No | `false` | Send StackStateChange alerts |
| `deployed` | `bool` | No | `false` | Triggers `DeployStack` (git pull + `compose up`) on create/update. Runs before `started`. |
| `started` | `bool` | No | `false` | When `true`, runs `compose up`. When `false`, runs `compose down`. Skipped if `deployed = true`. |
| `tags` | `list(string)` | No | `null` | Tag names to apply. `null` = unmanaged; `[]` = clear all tags. |

### komodo_deployment

Manages a Komodo deployment (a single Docker container).

> **server_id**: Omit to use the Local server. Set to `komodo_server.<name>.id` to target a specific server. See [server\_id — Local vs Remote Servers](#server_id--local-vs-remote-servers).

```hcl
resource "komodo_server" "prod" {
  name    = "prod-01"
  address = "http://10.0.1.100:8120"
}

resource "komodo_deployment" "api" {
  name      = "api-service"
  server_id = komodo_server.prod.id   # omit to use Local

  image_type = "Image"          # "Image" (default) or "Build"
  image      = "nginx:latest"   # docker image, or build ID when image_type = "Build"

  network      = "web"
  restart_mode = "unless-stopped"
  command      = ""

  ports       = "8080:80\n8443:443"
  volumes     = "/host/data:/app/data"
  environment = "NODE_ENV=production\nPORT=80"
  labels      = "traefik.enable=true"
  extra_args  = ["--memory=512m"]

  send_alerts = true
  auto_update = false

  deployed = true   # pull image + docker run on create/update
  started  = true   # docker run if deployed is false; docker stop+rm if false
}
```

#### Using a Komodo Build as the image source

```hcl
resource "komodo_deployment" "api" {
  name       = "api-service"
  server_id  = komodo_server.prod.id
  image_type = "Build"
  image      = komodo_build.api.id   # reference a komodo_build resource
  ...
}
```

#### Attributes

| Attribute | Type | Required | Default | Description |
| --------- | ---- | -------- | ------- | ----------- |
| `name` | `string` | Yes | - | Display name of the deployment |
| `server_id` | `string` | No | `"Local"` | Server to deploy on. Use `komodo_server.<name>.id`. Omit for Local. |
| `image_type` | `string` | No | `"Image"` | Image source: `"Image"` for a docker image, `"Build"` for a Komodo Build |
| `image` | `string` | No | `""` | Docker image (e.g. `nginx:latest`) when `image_type = "Image"`, or build ID when `image_type = "Build"` |
| `network` | `string` | No | API default | Docker network to connect to |
| `restart_mode` | `string` | No | API default (`"no"`) | Restart policy: `unless-stopped`, `always`, `on-failure`, `no` |
| `command` | `string` | No | `""` | Override the container entrypoint command |
| `ports` | `string` | No | `""` | Newline-separated port mappings (e.g. `"8080:80\n9090:90"`) |
| `volumes` | `string` | No | `""` | Newline-separated volume mounts (e.g. `"/host/path:/container/path"`) |
| `environment` | `string` | No | `""` | Newline-separated `KEY=VALUE` environment variables |
| `labels` | `string` | No | `""` | Newline-separated `KEY=VALUE` container labels |
| `extra_args` | `list(string)` | No | `null` | Extra arguments for `docker run` |
| `send_alerts` | `bool` | No | `false` | Send deployment state change alerts |
| `auto_update` | `bool` | No | `false` | Redeploy automatically when newer images are found |
| `deployed` | `bool` | No | `false` | Triggers `Deploy` (pull + `docker run`) on create/update. Runs before `started`. |
| `started` | `bool` | No | `false` | When `true`, runs `Deploy`. When `false`, runs `DestroyDeployment` (stop + rm). Skipped if `deployed = true`. |
| `tags` | `list(string)` | No | `null` | Tag names to apply. `null` = unmanaged; `[]` = clear all tags. |

### komodo_build

Manages a Komodo build configuration for Docker image builds.

```hcl
resource "komodo_build" "api" {
  name       = "api-service-build"
  builder_id = komodo_builder.build_server.id

  repo            = "myorg/api-service"
  branch          = "main"
  git_provider    = "github.com"
  git_account     = "myorg"
  dockerfile_path = "Dockerfile"
  build_path      = "."
  build_args = {
    NODE_ENV = "production"
    VERSION  = "1.0.0"
  }

  auto_increment_version = true
  webhook_enabled        = true
  send_alerts            = true
}
```

#### Attributes

| Attribute | Type | Required | Default | Description |
| --------- | ---- | -------- | ------- | ----------- |
| `name` | `string` | Yes | - | Display name of the build |
| `builder_id` | `string` | No | `""` | Builder to use. Use `komodo_builder.<name>.id`. |
| `image_name` | `string` | No | `""` | Alternate image name for the registry |
| `image_tag` | `string` | No | `""` | Extra tag appended after the build version |
| `auto_increment_version` | `bool` | No | `true` | Auto-increment patch version on every build |
| `git_provider` | `string` | No | `"github.com"` | Git provider domain |
| `git_https` | `bool` | No | `true` | Use HTTPS to clone |
| `git_account` | `string` | No | `""` | Git account for private repos |
| `repo` | `string` | No | `""` | Git repository in `namespace/repo_name` format |
| `branch` | `string` | No | `""` | Branch of the repo |
| `commit` | `string` | No | `""` | Pin to a specific commit hash |
| `build_path` | `string` | No | `""` | Build context path |
| `dockerfile_path` | `string` | No | `""` | Path to the Dockerfile |
| `use_buildx` | `bool` | No | `false` | Use `docker buildx` |
| `build_args` | `string` | No | `""` | Newline-separated `KEY=VALUE` build arguments |
| `labels` | `string` | No | `""` | Newline-separated `KEY=VALUE` image labels |
| `webhook_enabled` | `bool` | No | `false` | Allow incoming webhooks to trigger builds |
| `files_on_host` | `bool` | No | `false` | Source Dockerfile from the server filesystem |
| `tags` | `list(string)` | No | `null` | Tag names to apply. `null` = unmanaged; `[]` = clear all tags. |

### komodo_builder

Manages a Komodo builder (a server designated for building Docker images).

> **server_id**: Omit to use the Local server. Set to `komodo_server.<name>.id` to target a specific server.

```hcl
resource "komodo_builder" "build_server" {
  name      = "build-server-01"
  server_id = komodo_server.prod.id   # omit to use Local
}
```

#### Attributes

| Attribute | Type | Required | Default | Description |
| --------- | ---- | -------- | ------- | ----------- |
| `name` | `string` | Yes | - | Display name of the builder |
| `server_id` | `string` | No | `""` | Server to use as builder. Use `komodo_server.<name>.id`. Omit for Local. |
| `tags` | `list(string)` | No | `null` | Tag names to apply. `null` = unmanaged; `[]` = clear all tags. |

### komodo_repo

Manages a Komodo repository (a git repository cloned on a server).

> **server_id**: Omit to use the Local server. Set to `komodo_server.<name>.id` to target a specific server.

```hcl
resource "komodo_repo" "app_source" {
  name         = "app-source"
  server_id    = komodo_server.prod.id   # omit to use Local

  repo         = "myorg/app"
  branch       = "main"
  git_provider = "github.com"
  git_account  = "myorg"

  on_clone = "npm install"
  on_pull  = "npm install && npm run build"
}
```

#### Attributes

| Attribute | Type | Required | Default | Description |
| --------- | ---- | -------- | ------- | ----------- |
| `name` | `string` | Yes | - | Display name of the repo |
| `server_id` | `string` | No | `""` | Server to clone on. Use `komodo_server.<name>.id`. Omit for Local. |
| `repo` | `string` | No | `""` | Git repository in `namespace/repo_name` format |
| `branch` | `string` | No | `""` | Branch of the repo |
| `git_provider` | `string` | No | `"github.com"` | Git provider domain |
| `git_account` | `string` | No | `""` | Git account for private repos |
| `on_clone` | `string` | No | `""` | Command to run after initial clone |
| `on_pull` | `string` | No | `""` | Command to run after each pull |
| `webhook_enabled` | `bool` | No | `false` | Enable webhooks |
| `tags` | `list(string)` | No | `null` | Tag names to apply. `null` = unmanaged; `[]` = clear all tags. |

### komodo_tag

Manages a Komodo tag (a label for organizing resources).

```hcl
resource "komodo_tag" "production" {
  name = "production"
}
```

#### Attributes

| Attribute | Type | Required | Default | Description |
| --------- | ---- | -------- | ------- | ----------- |
| `name` | `string` | Yes | - | Name of the tag |

### komodo_user

Manages a Komodo user. Supports `local` (password-based) and `service` (API key-based) user types.

```hcl
# Service user (for CI pipelines / automated access)
resource "komodo_user" "ci_bot" {
  username  = "ci-bot"
  user_type = "service"
  enabled   = true

  create_server_permissions = false
  create_build_permissions  = true
}

# Local user (password login)
resource "komodo_user" "operator" {
  username  = "operator"
  user_type = "local"
  password  = var.operator_password
  enabled   = true
}
```

#### Attributes

| Attribute | Type | Required | Default | Description |
| --------- | ---- | -------- | ------- | ----------- |
| `username` | `string` | Yes | - | Globally unique username. Changing this forces a new resource. |
| `user_type` | `string` | No | `"service"` | `"local"` or `"service"`. Changing this forces a new resource. |
| `description` | `string` | No | `""` | Description (service users only) |
| `password` | `string` | No | - | Password (local users only). Sensitive. |
| `enabled` | `bool` | No | `true` | Whether the user can access the API |
| `admin` | `bool` | No | `false` | Grant global admin permissions |
| `create_server_permissions` | `bool` | No | `false` | Allow the user to create servers |
| `create_build_permissions` | `bool` | No | `false` | Allow the user to create builds |

### komodo_git_provider_account

Manages a Komodo git provider account (credentials for cloning private repositories). The `domain` + `username` combination must be unique.

```hcl
resource "komodo_git_provider_account" "gitea" {
  domain   = "gitea.example.com"
  username = "my-gitea-user"
  token    = var.gitea_token
  https    = true
}

resource "komodo_git_provider_account" "github" {
  domain   = "github.com"
  username = "my-github-user"
  token    = var.github_pat
  https    = true
}
```

#### Attributes

| Attribute | Type | Required | Default | Description |
| --------- | ---- | -------- | ------- | ----------- |
| `domain` | `string` | Yes | - | Git provider domain (e.g. `github.com`). Changing this forces a new resource. |
| `username` | `string` | Yes | - | Git account username. Changing this forces a new resource. |
| `token` | `string` | Yes | - | Personal access token or password. Sensitive. |
| `https` | `bool` | No | `true` | Use HTTPS (`true`) or HTTP (`false`) |

> **Note**: The token is write-only — it is not returned by the API on reads. Import with `terraform import komodo_git_provider_account.<name> <id>`.

### komodo_api_key

Manages an API key for a Komodo service user. The secret is only returned at creation time.

```hcl
resource "komodo_api_key" "ci_bot_key" {
  user_id = komodo_user.ci_bot.id
  name    = "ci-pipeline-key"
}

output "ci_bot_api_key" {
  value     = komodo_api_key.ci_bot_key.key
  sensitive = true
}

output "ci_bot_api_secret" {
  value     = komodo_api_key.ci_bot_key.secret
  sensitive = true
}
```

#### Attributes

| Attribute | Type | Required | Default | Description |
| --------- | ---- | -------- | ------- | ----------- |
| `user_id` | `string` | Yes | - | ID of the service user. Changing this forces a new resource. |
| `name` | `string` | Yes | - | Name for the API key. Changing this forces a new resource. |
| `key` | `string` | Computed | - | The API key (`K-...`). Sensitive. |
| `secret` | `string` | Computed | - | The API secret (`S-...`). Only available at creation time. Sensitive. |

## Data Sources

Each resource has a corresponding read-only data source for looking up existing resources by ID or name:

```hcl
data "komodo_server" "web" {
  id = "web-01"   # accepts the server's display name or its canonical ID
}

output "server_address" {
  value = data.komodo_server.web.address
}
```

Available data sources: `komodo_server`, `komodo_stack`, `komodo_deployment`, `komodo_build`, `komodo_builder`, `komodo_repo`, `komodo_tag`.

## Examples

### Complete setup: server + stack + deployment

```hcl
terraform {
  required_providers {
    komodo = {
      source  = "registry.terraform.io/moghtech/komodo"
      version = "0.1.0"
    }
  }
}

provider "komodo" {
  base_url   = var.komodo_url
  api_key    = var.komodo_api_key
  api_secret = var.komodo_api_secret
}

# Tags for organising resources
resource "komodo_tag" "production" {
  name = "production"
}

resource "komodo_tag" "team_backend" {
  name = "backend"
}

# Register the server with Komodo
resource "komodo_server" "prod" {
  name    = "prod-01"
  address = "http://10.0.1.100:8120"
  enabled = true
  tags    = [komodo_tag.production.name]

  stats_monitoring = true
  cpu_warning      = 80
  cpu_critical     = 95
}

# Git-based stack on the registered server
resource "komodo_stack" "web" {
  name      = "web-app"
  server_id = komodo_server.prod.id
  tags      = [komodo_tag.production.name, komodo_tag.team_backend.name]

  git_provider = "github.com"
  git_account  = "myorg"
  repo         = "myorg/web-app"
  branch       = "main"
  file_paths   = ["docker-compose.yml"]

  auto_pull   = true
  send_alerts = true
  environment = "NODE_ENV=production"

  deployed = true
  started  = true
}

# Single-container deployment on the same server
resource "komodo_deployment" "api" {
  name      = "api-service"
  server_id = komodo_server.prod.id
  tags      = [komodo_tag.production.name, komodo_tag.team_backend.name]

  image        = "myregistry/api-service:latest"
  restart_mode = "unless-stopped"
  ports        = "8080:8080"
  environment  = "NODE_ENV=production\nDATABASE_URL=${var.database_url}"

  send_alerts = true
  auto_update = true
  deployed    = true
  started     = true
}
```

### Inline stack on Local (no server resource needed)

```hcl
resource "komodo_stack" "hello" {
  name = "hello-world"
  # server_id omitted → deploys on the Local server

  file_contents = <<-EOT
    services:
      hello:
        image: hello-world:latest
  EOT

  deployed = true
  started  = true
}
```

### Server managed elsewhere (use data source)

```hcl
# Look up a server that exists in Komodo but isn't managed by this config
data "komodo_server" "existing" {
  id = "prod-01"   # the server's display name as set in the Komodo UI
}

resource "komodo_deployment" "api" {
  name      = "api-service"
  server_id = data.komodo_server.existing.id

  image   = "nginx:latest"
  started = true
}
```

## Import

All resources support `terraform import` by their Komodo ID:

```bash
terraform import komodo_server.prod     66113df3abe32960b87018dd
terraform import komodo_stack.web       67076689ed600cfdd52ac637
terraform import komodo_deployment.api  67a1b2c3d4e5f6a7b8c9d0e1
```

## Development

### Building from Source

```bash
git clone https://github.com/johnciavarella/terraform-provider-komodo
cd terraform-provider-komodo
go mod tidy
make install
```

### Running Tests

```bash
# Unit tests
make test

# Acceptance tests (requires a live Komodo instance)
export KOMODO_ADDRESS=https://komodo.example.com
export KOMODO_API_KEY=your_key
export KOMODO_API_SECRET=your_secret
make testacc
```

### Debug Logging

```bash
export TF_LOG=DEBUG
terraform apply
```

## Troubleshooting

| Problem | Fix |
| ------- | --- |
| Connection refused | Verify Komodo Core and Periphery are running and reachable |
| Authentication failure | Check API key and secret |
| Resource not found | Confirm referenced resources (servers, builders) exist in Komodo |
| Permission denied | Ensure the API credentials have sufficient permissions |

## API Reference

This provider wraps the Komodo Core RPC-style HTTP API. All requests are `POST` with JSON bodies to `/read`, `/write`, or `/execute`. See the [full API docs](https://docs.rs/komodo_client/latest/komodo_client/api/index.html).

## License

GPL-3.0-or-later (matching the Komodo project license)
