package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type fakeS3 struct {
	listBuckets   func(context.Context, *s3.ListBucketsInput) (*s3.ListBucketsOutput, error)
	listObjectsV2 func(context.Context, *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error)
}

func (fake fakeS3) ListBuckets(
	ctx context.Context,
	input *s3.ListBucketsInput,
	_ ...func(*s3.Options),
) (*s3.ListBucketsOutput, error) {
	return fake.listBuckets(ctx, input)
}

func (fake fakeS3) ListObjectsV2(
	ctx context.Context,
	input *s3.ListObjectsV2Input,
	_ ...func(*s3.Options),
) (*s3.ListObjectsV2Output, error) {
	return fake.listObjectsV2(ctx, input)
}

func TestNormalizeEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "host and port", input: "minio:9000", want: "http://minio:9000"},
		{name: "HTTP URL", input: "http://localhost:9000/", want: "http://localhost:9000"},
		{name: "HTTPS URL", input: "https://minio.example.com", want: "https://minio.example.com"},
		{name: "missing", wantErr: true},
		{name: "unsupported scheme", input: "ftp://minio:9000", wantErr: true},
		{name: "query", input: "https://minio.example.com?debug=1", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeEndpoint(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("normalizeEndpoint(%q) returned no error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeEndpoint(%q): %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("normalizeEndpoint(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRouter(t *testing.T) {
	ginTestMode(t)

	client := fakeS3{
		listBuckets: func(context.Context, *s3.ListBucketsInput) (*s3.ListBucketsOutput, error) {
			return &s3.ListBucketsOutput{Buckets: []types.Bucket{
				{Name: aws.String("raw-data")},
				{Name: aws.String("processed-data")},
			}}, nil
		},
		listObjectsV2: func(_ context.Context, input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error) {
			if got := aws.ToString(input.Bucket); got != "raw-data" {
				t.Fatalf("bucket = %q, want raw-data", got)
			}
			return &s3.ListObjectsV2Output{Contents: []types.Object{
				{Key: aws.String("events/one.json")},
				{Key: aws.String("events/two.json")},
			}}, nil
		},
	}
	router := newRouter(client)

	tests := []struct {
		path string
		want string
	}{
		{path: "/healthz", want: `{"status":"ok"}`},
		{path: "/buckets", want: `{"buckets":["raw-data","processed-data"]}`},
		{path: "/buckets/raw-data/objects", want: `{"objects":["events/one.json","events/two.json"]}`},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
			}
			if got := strings.TrimSpace(response.Body.String()); got != tt.want {
				t.Fatalf("body = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRouterDoesNotExposeUpstreamError(t *testing.T) {
	ginTestMode(t)

	const sensitiveMessage = "upstream response contains a credential"
	client := fakeS3{
		listBuckets: func(context.Context, *s3.ListBucketsInput) (*s3.ListBucketsOutput, error) {
			return nil, errors.New(sensitiveMessage)
		},
		listObjectsV2: func(context.Context, *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error) {
			return nil, errors.New(sensitiveMessage)
		},
	}
	router := newRouter(client)

	for _, path := range []string{"/buckets", "/buckets/raw-data/objects"} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if response.Code != http.StatusBadGateway {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadGateway)
			}
			if strings.Contains(response.Body.String(), sensitiveMessage) {
				t.Fatalf("response exposed upstream error: %s", response.Body.String())
			}
		})
	}
}

func ginTestMode(t *testing.T) {
	t.Helper()
	t.Setenv("GIN_MODE", "test")
}
