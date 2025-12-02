package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
)

var s3Client *s3.Client

func main() {
	// 1. Minio 接続設定の初期化
	initS3Client()

	// 2. Gin ルーターのセットアップ
	r := gin.Default()

	// 3. ルート定義
	r.GET("/buckets", listBuckets)
	r.GET("/buckets/:name/objects", listObjects)

	// 4. サーバー起動
	r.Run(":8080")
}

func initS3Client() {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	accessKey := os.Getenv("MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MINIO_SECRET_KEY")

	// 最小限のエラーハンドリング
	if endpoint == "" || accessKey == "" || secretKey == "" {
		fmt.Println("Error: MINIO environment variables are missing")
		os.Exit(1)
	}

	// カスタムエンドポイント（Minio用）の設定
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		panic(err)
	}

	// PathStyle を有効にし、カスタムエンドポイントを設定するのが Minio 接続のポイントです
	s3Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("http://" + endpoint)
		o.UsePathStyle = true
	})
}

// バケット一覧取得
func listBuckets(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	output, err := s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var buckets []string
	for _, b := range output.Buckets {
		buckets = append(buckets, *b.Name)
	}
	c.JSON(http.StatusOK, gin.H{"buckets": buckets})
}

// オブジェクト一覧取得
func listObjects(c *gin.Context) {
	bucketName := c.Param("name")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	output, err := s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var objects []string
	for _, obj := range output.Contents {
		objects = append(objects, *obj.Key)
	}
	c.JSON(http.StatusOK, gin.H{"objects": objects})
}
