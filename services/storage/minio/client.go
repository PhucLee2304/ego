package minio

import (
	"context"
	"ego/services/storage/config"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	s3Config "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

type Client struct {
	Client         *s3.Client
	PresignClient  *s3.PresignClient
	Bucket         string
	Endpoint       string
	Host           string
	PublicEndpoint string
	Expiry         time.Duration
}

func NewS3Client(cfg *config.AppConfig) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	awsCfg, err := s3Config.LoadDefaultConfig(ctx,
		s3Config.WithRegion(cfg.R2Region),
		s3Config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.R2AccessKey,
			cfg.R2SecretKey,
			"",
		)),
	)
	if err != nil {
		return nil, err
	}

	internalS3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String(cfg.R2Host)
	})

	publicS3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String(cfg.R2Endpoint)
	})

	presignClient := s3.NewPresignClient(publicS3Client)
	if err := ensureBucket(ctx, internalS3Client, cfg.R2Bucket); err != nil {
		return nil, fmt.Errorf("failed to ensure bucket %q: %w", cfg.R2Bucket, err)
	}
	if cfg.R2PublicRead {
		if err := ensurePublicRead(ctx, internalS3Client, cfg.R2Bucket); err != nil {
			return nil, fmt.Errorf("failed to make bucket %q public: %w", cfg.R2Bucket, err)
		}
	}

	return &Client{
		Client:         internalS3Client,
		PresignClient:  presignClient,
		Bucket:         cfg.R2Bucket,
		Endpoint:       cfg.R2Endpoint,
		Host:           cfg.R2Host,
		PublicEndpoint: cfg.R2PublicEndpoint,
		Expiry:         cfg.R2Expiry,
	}, nil
}

func ensurePublicRead(ctx context.Context, client *s3.Client, bucket string) error {
	policy, err := json.Marshal(map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{
			{
				"Effect":    "Allow",
				"Principal": "*",
				"Action":    []string{"s3:GetObject"},
				"Resource":  []string{fmt.Sprintf("arn:aws:s3:::%s/*", bucket)},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("build public-read policy: %w", err)
	}

	_, err = client.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: aws.String(bucket),
		Policy: aws.String(string(policy)),
	})
	if err != nil {
		return fmt.Errorf("put public-read policy: %w", err)
	}
	return nil
}

func ensureBucket(ctx context.Context, client *s3.Client, bucket string) error {
	_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	if err == nil {
		return nil
	}

	var responseErr *smithyhttp.ResponseError
	isNotFound := errors.As(err, &responseErr) && responseErr.HTTPStatusCode() == 404
	if !isNotFound {
		var apiErr smithy.APIError
		isNotFound = errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NotFound" || apiErr.ErrorCode() == "NoSuchBucket")
	}
	if !isNotFound {
		return fmt.Errorf("check bucket: %w", err)
	}

	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	if err == nil {
		return nil
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && apiErr.ErrorCode() == "BucketAlreadyOwnedByYou" {
		return nil
	}

	return fmt.Errorf("create bucket: %w", err)
}
