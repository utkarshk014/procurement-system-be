// Package storage keeps uploaded documents in S3 (or an S3-compatible store such as MinIO in
// tests). The bucket stays private: files are read through short-lived signed links.
package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Config says where files go. Credentials come from AWS_ACCESS_KEY_ID and
// AWS_SECRET_ACCESS_KEY (the SDK's usual chain) unless AccessKey is set.
type S3Config struct {
	Bucket    string
	Region    string
	Endpoint  string // empty for AWS; a URL for MinIO or another S3-compatible store
	AccessKey string
	SecretKey string
}

// S3 stores files in one bucket.
type S3 struct {
	bucket  string
	client  *s3.Client
	presign *s3.PresignClient
}

// NewS3 connects to the bucket. It does not call S3: a wrong key shows on the first upload.
func NewS3(ctx context.Context, cfg S3Config) (*S3, error) {
	opts := []func(*config.LoadOptions) error{config.WithRegion(cfg.Region)}
	if cfg.AccessKey != "" {
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")))
	}
	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("s3 config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		}
	})
	return &S3{bucket: cfg.Bucket, client: client, presign: s3.NewPresignClient(client)}, nil
}

// Put uploads body (size bytes) under key. The body is read into memory first (files are
// small): the SDK signs a checksum of the content and needs to read it twice.
func (s *S3) Put(ctx context.Context, key, contentType string, size int64, body io.Reader) error {
	data, err := io.ReadAll(io.LimitReader(body, size+1))
	if err != nil {
		return fmt.Errorf("read %s: %w", key, err)
	}
	if int64(len(data)) != size {
		return fmt.Errorf("read %s: got %d bytes, expected %d", key, len(data), size)
	}
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(data),
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("s3 put %s: %w", key, err)
	}
	return nil
}

// Delete removes key; a key that is already gone is not an error.
func (s *S3) Delete(ctx context.Context, key string) error {
	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)}); err != nil {
		return fmt.Errorf("s3 delete %s: %w", key, err)
	}
	return nil
}

// URL returns a signed link that downloads key as downloadName for ttl.
func (s *S3) URL(ctx context.Context, key, downloadName string, ttl time.Duration) (string, error) {
	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     aws.String(s.bucket),
		Key:                        aws.String(key),
		ResponseContentDisposition: aws.String(mime.FormatMediaType("inline", map[string]string{"filename": downloadName})),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("s3 presign %s: %w", key, err)
	}
	return req.URL, nil
}
