package minio

import (
	"context"
	"log"
	"crypto/tls"
    "net/http"
	"github.com/linskybing/platform-go/config"
	minioSDK "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var Client *minioSDK.Client
var BucketName string

func InitMinio() {
	endpoint := config.MinioEndpoint
	accessKey := config.MinioAccessKey
	secretKey := config.MinioSecretKey
	useSSL := config.MinioUseSSL
	BucketName = config.MinioBucket

	// Initialize MinIO client
    transport := &http.Transport{
        TLSClientConfig: &tls.Config{
            InsecureSkipVerify: true,
        },
    }

    // Initialize MinIO client with custom transport
    minioClient, err := minioSDK.New(endpoint, &minioSDK.Options{
        Creds:     credentials.NewStaticV4(accessKey, secretKey, ""),
        Secure:    useSSL,
        Transport: transport,
    })
	if err != nil {
		log.Fatalf("❌ Failed to connect to MinIO: %v", err)
	}

	log.Println("✅ Successfully connected to MinIO")

	// Check if bucket exists, create if not
	ctx := context.Background()
	exists, err := minioClient.BucketExists(ctx, BucketName)
	if err != nil {
		log.Fatalf("❌ Failed to check bucket existence: %v", err)
	}

	if !exists {
		err := minioClient.MakeBucket(ctx, BucketName, minioSDK.MakeBucketOptions{})
		if err != nil {
			log.Fatalf("❌ Failed to create bucket: %v", err)
		}
		log.Printf("✅ Bucket created: %s\n", BucketName)
	} else {
		log.Printf("ℹ️ Bucket already exists: %s\n", BucketName)
	}

	Client = minioClient
}
