# simple-go-app

Simple Go application that lists Minio/S3 buckets and objects via HTTP endpoints.

## Local Development

Run the server locally (set Minio env vars first):

```bash
export MINIO_ENDPOINT=localhost:9000
export MINIO_ACCESS_KEY=minioadmin
export MINIO_SECRET_KEY=minioadmin
go run main.go
```

## API Endpoints
- `GET /buckets` — list buckets
- `GET /buckets/:name/objects` — list objects in a bucket

## CI/CD & GitOps Deployment

This repository uses GitHub Actions for automated builds and GitOps deployment to Kubernetes.

### Workflow Overview

1. **Build & Push**: On push to `main`, the workflow builds multi-arch (amd64/arm64) container images and pushes them to GitHub Container Registry (ghcr.io)
2. **Update Manifest**: Automatically updates the image tag in the `learn-k8s` repository's Kustomize overlay
3. **ArgoCD Sync**: ArgoCD detects the manifest change and deploys to Kubernetes

### Required Secret

For GitOps automation to work, you need to configure a Personal Access Token:

1. Create a PAT with `repo` scope at: https://github.com/settings/tokens
2. Add it as a repository secret named `PAT_FOR_GITOPS`
3. Settings > Secrets and variables > Actions > New repository secret

### Image Tags

- `ghcr.io/uchida-abeja/simple-go-app:latest` — latest build from main branch
- `ghcr.io/uchida-abeja/simple-go-app:<short-sha>` — specific commit (used in Kubernetes)

### Deployment Location

Kubernetes manifests are managed in: `uchida-abeja/learn-k8s` repository
- Base: `apps/simple-go-app/base/`
- Dev overlay: `apps/simple-go-app/overlays/dev/`
