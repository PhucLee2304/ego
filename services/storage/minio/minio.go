package minio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func (c *Client) GeneratePresignedUploadURL(ctx context.Context, folders []BucketFolder, fileName string, contentType string) (*string, error) {
	objectKey := BuildObjectKey(folders, fileName)
	if objectKey == "" {
		return nil, errors.New("object key cannot be empty")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	request, err := c.PresignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.Bucket),
		Key:         aws.String(objectKey),
		ContentType: aws.String(contentType),
	},
		func(options *s3.PresignOptions) {
			options.Expires = c.Expiry
		},
	)
	if err != nil {
		return nil, err
	}

	return &request.URL, nil
}

func (c *Client) GeneratePresignedDownloadURL(ctx context.Context, folders []BucketFolder, fileName string) (*string, error) {
	objectKey := BuildObjectKey(folders, fileName)
	if objectKey == "" {
		return nil, errors.New("object key cannot be empty")
	}

	request, err := c.PresignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(objectKey),
	},
		func(options *s3.PresignOptions) {
			options.Expires = c.Expiry
		},
	)
	if err != nil {
		return nil, err
	}

	return &request.URL, nil
}

func (c *Client) UploadFile(
	ctx context.Context,
	folders []BucketFolder,
	fileName string,
	reader io.Reader,
	size int64,
	contentType string,
) (*string, error) {
	objectKey := BuildObjectKey(folders, fileName)
	if objectKey == "" {
		return nil, errors.New("object key cannot be empty")
	}
	if reader == nil {
		return nil, errors.New("reader cannot be nil")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err := c.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.Bucket),
		Key:           aws.String(objectKey),
		Body:          reader,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	url := fmt.Sprintf("%s/%s/%s", strings.TrimRight(c.Endpoint, "/"), c.Bucket, objectKey)
	return &url, nil
}

func (c *Client) DownloadFile(ctx context.Context, folders []BucketFolder, fileName string) (io.ReadCloser, error) {
	objectKey := BuildObjectKey(folders, fileName)
	url := fmt.Sprintf("%s/%s", strings.TrimRight(c.Host, "/"), objectKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		err := resp.Body.Close()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("failed to download file, status: %s", resp.Status)
	}

	return resp.Body, nil
}
