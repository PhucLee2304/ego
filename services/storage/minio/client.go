package minio

import (
	"context"
	"ego/services/storage/config"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	s3Config "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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
