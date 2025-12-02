# simple-go-app

Simple Go application that lists Minio/S3 buckets and objects via HTTP endpoints.

Run the server locally (set Minio env vars first):

```bash
export MINIO_ENDPOINT=localhost:9000
export MINIO_ACCESS_KEY=minioadmin
export MINIO_SECRET_KEY=minioadmin
go run main.go
```

Endpoints:
- `GET /buckets` — list buckets
- `GET /buckets/:name/objects` — list objects in a bucket
