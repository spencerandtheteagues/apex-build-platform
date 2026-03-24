package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// R2Provider implements storage using Cloudflare R2
type R2Provider struct {
	client       *s3.Client
	presigner    *s3.PresignClient
	uploader     *manager.Uploader
	bucket       string
	endpoint     string
}

// NewR2Provider creates a new R2 storage provider
func NewR2Provider(accountID, accessKeyID, secretAccessKey, bucketName string) (*R2Provider, error) {
	if accountID == "" || accessKeyID == "" || secretAccessKey == "" || bucketName == "" {
		return nil, fmt.Errorf("R2 credentials incomplete: account_id=%q access_key_id=%q secret_access_key=%q bucket_name=%q",
			accountID, accessKeyID != "", secretAccessKey != "", bucketName)
	}

	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)

	// Create custom resolver for R2 endpoint
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if service == s3.ServiceID {
			return aws.Endpoint{
				URL:               endpoint,
				SigningRegion:     "auto",
				HostnameImmutable: true,
			}, nil
		}
		return aws.Endpoint{}, fmt.Errorf("unknown service: %s", service)
	})

	// Create AWS config with R2 credentials and endpoint
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accessKeyID,
			secretAccessKey,
			"",
		)),
		config.WithEndpointResolverWithOptions(customResolver),
		config.WithRegion("auto"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load R2 config: %w", err)
	}

	// Create S3 client
	client := s3.NewFromConfig(cfg)

	// Create presigner
	presigner := s3.NewPresignClient(client)

	// Create uploader
	uploader := manager.NewUploader(client)

	return &R2Provider{
		client:    client,
		presigner: presigner,
		uploader:  uploader,
		bucket:    bucketName,
		endpoint:  endpoint,
	}, nil
}

// Put uploads content to R2
func (r *R2Provider) Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	input := &s3.PutObjectInput{
		Bucket:        aws.String(r.bucket),
		Key:           aws.String(key),
		Body:          reader,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	}

	_, err := r.uploader.Upload(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to upload to R2: %w", err)
	}

	return nil
}

// Get downloads content from R2
func (r *R2Provider) Get(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	}

	result, err := r.client.GetObject(ctx, input)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get from R2: %w", err)
	}

	size := int64(0)
	if result.ContentLength != nil {
		size = *result.ContentLength
	}

	return result.Body, size, nil
}

// Delete removes an object from R2
func (r *R2Provider) Delete(ctx context.Context, key string) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	}

	_, err := r.client.DeleteObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete from R2: %w", err)
	}

	return nil
}

// URL returns a presigned URL for the object
func (r *R2Provider) URL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	}

	req, err := r.presigner.PresignGetObject(ctx, input, func(opts *s3.PresignOptions) {
		opts.Expires = ttl
	})
	if err != nil {
		return "", fmt.Errorf("failed to presign R2 URL: %w", err)
	}

	return req.URL, nil
}

// Exists checks if an object exists in R2
func (r *R2Provider) Exists(ctx context.Context, key string) (bool, error) {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	}

	_, err := r.client.HeadObject(ctx, input)
	if err != nil {
		var notFound *types.NotFound
		if errors.As(err, &notFound) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check R2 object existence: %w", err)
	}

	return true, nil
}