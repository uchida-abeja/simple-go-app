package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
)

const (
	defaultPort        = "8080"
	defaultRegion      = "us-east-1"
	requestTimeout     = 30 * time.Second
	readHeaderTimeout  = 5 * time.Second
	serverIdleTimeout  = 60 * time.Second
	serverWriteTimeout = 35 * time.Second
)

type appConfig struct {
	endpoint  string
	accessKey string
	secretKey string
	region    string
	port      string
}

type s3API interface {
	ListBuckets(context.Context, *s3.ListBucketsInput, ...func(*s3.Options)) (*s3.ListBucketsOutput, error)
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

type application struct {
	s3 s3API
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := configFromEnv()
	if err != nil {
		return err
	}

	client, err := newS3Client(cfg)
	if err != nil {
		return fmt.Errorf("initialize S3 client: %w", err)
	}

	server := &http.Server{
		Addr:              ":" + cfg.port,
		Handler:           newRouter(client),
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      serverWriteTimeout,
		IdleTimeout:       serverIdleTimeout,
	}

	log.Printf("listening on %s", server.Addr)
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	return nil
}

func configFromEnv() (appConfig, error) {
	endpoint, err := normalizeEndpoint(os.Getenv("MINIO_ENDPOINT"))
	if err != nil {
		return appConfig{}, fmt.Errorf("MINIO_ENDPOINT: %w", err)
	}

	accessKey := strings.TrimSpace(os.Getenv("MINIO_ACCESS_KEY"))
	if accessKey == "" {
		return appConfig{}, errors.New("MINIO_ACCESS_KEY is required")
	}

	secretKey := strings.TrimSpace(os.Getenv("MINIO_SECRET_KEY"))
	if secretKey == "" {
		return appConfig{}, errors.New("MINIO_SECRET_KEY is required")
	}

	region := strings.TrimSpace(os.Getenv("AWS_REGION"))
	if region == "" {
		region = defaultRegion
	}

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = defaultPort
	}

	return appConfig{
		endpoint:  endpoint,
		accessKey: accessKey,
		secretKey: secretKey,
		region:    region,
		port:      port,
	}, nil
}

func normalizeEndpoint(raw string) (string, error) {
	endpoint := strings.TrimSpace(raw)
	if endpoint == "" {
		return "", errors.New("is required")
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "http://" + endpoint
	}

	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", errors.New("must be a valid HTTP(S) URL or host:port")
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", errors.New("must use http or https and include a host")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("must not contain a query or fragment")
	}

	return strings.TrimRight(endpoint, "/"), nil
}

func newS3Client(cfg appConfig) (*s3.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	awsCfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(cfg.region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.accessKey, cfg.secretKey, ""),
		),
	)
	if err != nil {
		return nil, err
	}

	return s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(cfg.endpoint)
		options.UsePathStyle = true
	}), nil
}

func newRouter(client s3API) http.Handler {
	app := application{s3: client}
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/buckets", app.listBuckets)
	router.GET("/buckets/:name/objects", app.listObjects)
	return router
}

func (app application) listBuckets(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), requestTimeout)
	defer cancel()

	output, err := app.s3.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		log.Printf("list buckets: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to list buckets"})
		return
	}

	buckets := make([]string, 0, len(output.Buckets))
	for _, bucket := range output.Buckets {
		if name := aws.ToString(bucket.Name); name != "" {
			buckets = append(buckets, name)
		}
	}
	c.JSON(http.StatusOK, gin.H{"buckets": buckets})
}

func (app application) listObjects(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), requestTimeout)
	defer cancel()

	bucketName := c.Param("name")
	output, err := app.s3.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		log.Printf("list objects in bucket %q: %v", bucketName, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to list objects"})
		return
	}

	objects := make([]string, 0, len(output.Contents))
	for _, object := range output.Contents {
		if key := aws.ToString(object.Key); key != "" {
			objects = append(objects, key)
		}
	}
	c.JSON(http.StatusOK, gin.H{"objects": objects})
}
