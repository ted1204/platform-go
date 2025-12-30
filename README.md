
# Platform API (Go)

This project is a RESTful API written in Go. It uses Gin as the HTTP framework and integrates with PostgreSQL and MinIO. The repository includes Dockerfile, Kubernetes manifests under `k8s/`, and helper scripts in `scripts/`.

Prerequisites
- Go 1.21+
- PostgreSQL (or a running DB instance)
- (Optional) MinIO or S3-compatible storage for YAML/object storage

Quick start (local)

```bash
# fetch dependencies
go mod download

# run locally
go run src/main.go
```

Build Docker image

```bash
docker build -t platform-go:latest .
```

Helper scripts (in `scripts/`)
- `dev.sh` — start local development environment (check script for exact behavior)
- `build_images.sh` — build Docker images used by deployment
- `setup_fakek8s.sh` — helper to set up a fake/local k8s environment (read before use)
- `create_gpu_pod.py` — test GPU pod creation helper
- `reset-cluster.sh` not present here; use caution with scripts that modify cluster state

Kubernetes and infra
- `k8s/` contains example manifests for deploying the service
- `infra/` and `yamls/` contain environment-specific templates (volume claims, configs)

Project layout (important dirs)
- `src/` — application code (config, handlers, routes, services, models)
- `scripts/` — helper automation scripts
- `k8s/` & `k8s-project/` — manifests for cluster deployments
- `Dockerfile` — container build

Configuration
- Application reads configuration from environment variables or config files (check `src/config/` for keys).

Testing

```bash
cd src
go test ./...   # run unit tests
```

API docs
- Swagger/docs are generated under `src/docs` — run the server and open `/swagger.html`.

Notes
- Read each script before running; some scripts modify cluster or host configuration.
- If you want, I can add a short `dev.md` showing typical environment variables and a sample `docker-compose` for local development.
