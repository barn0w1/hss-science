package s3storage

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/barn0w1/hss-science/server/services/blob-service/internal/domain"
)

type R2Client struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
}

func New(endpoint, bucket, accessKeyID, secretKey string) (*R2Client, error) {
	creds := credentials.NewStaticCredentialsProvider(accessKeyID, secretKey, "")
	client := s3.New(s3.Options{
		BaseEndpoint: aws.String(endpoint),
		Credentials:  creds,
		Region:       "auto",
		UsePathStyle: true,
	})
	return &R2Client{
		client:  client,
		presign: s3.NewPresignClient(client),
		bucket:  bucket,
	}, nil
}

func (c *R2Client) PresignedPutURL(ctx context.Context, key string, ttl time.Duration) (string, time.Time, error) {
	req, err := c.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("r2: presign put: %w", err)
	}
	return req.URL, time.Now().Add(ttl), nil
}

func (c *R2Client) PresignedGetURL(ctx context.Context, key string, ttl time.Duration) (string, time.Time, error) {
	req, err := c.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("r2: presign get: %w", err)
	}
	return req.URL, time.Now().Add(ttl), nil
}

func (c *R2Client) CreateMultipartUpload(ctx context.Context, key, contentType string) (string, error) {
	out, err := c.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("r2: create multipart upload: %w", err)
	}
	return aws.ToString(out.UploadId), nil
}

func (c *R2Client) PresignedPartURL(ctx context.Context, key, uploadID string, partNumber int32, ttl time.Duration) (string, time.Time, error) {
	req, err := c.presign.PresignUploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(c.bucket),
		Key:        aws.String(key),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int32(partNumber),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("r2: presign part: %w", err)
	}
	return req.URL, time.Now().Add(ttl), nil
}

func (c *R2Client) CompleteMultipartUpload(ctx context.Context, key, uploadID string, parts []domain.CompletedPart) error {
	completed := make([]types.CompletedPart, len(parts))
	for i, p := range parts {
		completed[i] = types.CompletedPart{
			PartNumber: aws.Int32(p.PartNumber),
			ETag:       aws.String(p.ETag),
		}
	}
	_, err := c.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(c.bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completed,
		},
	})
	if err != nil {
		return fmt.Errorf("r2: complete multipart: %w", err)
	}
	return nil
}

func (c *R2Client) AbortMultipartUpload(ctx context.Context, key, uploadID string) error {
	_, err := c.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(c.bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	})
	if err != nil {
		return fmt.Errorf("r2: abort multipart: %w", err)
	}
	return nil
}
