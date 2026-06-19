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
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func (c *Client) GeneratePresignedUploadURL(ctx context.Context, folders []BucketFolder, fileName string, contentType string) (*string, error) {
	objectKey := BuildObjectKey(folders, fileName)
	if objectKey == "" {
		return nil, errors.New("[MINIO] Object key cannot be empty")
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
		return nil, errors.New("[MINIO] Object key cannot be empty")
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
		return nil, errors.New("[MINIO] Object key cannot be empty")
	}
	if reader == nil {
		return nil, errors.New("[MINIO] Reader cannot be nil")
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
		return nil, fmt.Errorf("[MINIO] Failed to upload file: %w", err)
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
		return nil, fmt.Errorf("[MINIO] Failed to download file, status: %s", resp.Status)
	}

	return resp.Body, nil
}

func (c *Client) GetPublicURL(folders []BucketFolder, fileName string) (*string, error) {
	objectKey := BuildObjectKey(folders, fileName)
	if objectKey == "" {
		return nil, errors.New("[MINIO] Object key cannot be empty")
	}
	if strings.TrimSpace(c.PublicEndpoint) == "" {
		return nil, errors.New("[MINIO] R2_PUBLIC_ENDPOINT is required to build a viewable public URL")
	}

	url := fmt.Sprintf("%s/%s", strings.TrimRight(c.PublicEndpoint, "/"), objectKey)
	return &url, nil
}

func (c *Client) DeleteFolder(ctx context.Context, folder string) (uint32, error) {
	prefix := strings.Trim(folder, "/")
	if prefix == "" {
		return 0, errors.New("[MINIO] Folder cannot be empty")
	}
	prefix += "/"

	var deletedCount uint32
	paginator := s3.NewListObjectsV2Paginator(c.Client, &s3.ListObjectsV2Input{
		Bucket: aws.String(c.Bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return deletedCount, fmt.Errorf("[MINIO] Failed to list objects for folder %q: %w", folder, err)
		}
		if len(page.Contents) == 0 {
			continue
		}

		objects := make([]types.ObjectIdentifier, 0, len(page.Contents))
		for _, object := range page.Contents {
			if object.Key == nil || *object.Key == "" {
				continue
			}
			objects = append(objects, types.ObjectIdentifier{
				Key: object.Key,
			})
		}
		if len(objects) == 0 {
			continue
		}

		output, err := c.Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(c.Bucket),
			Delete: &types.Delete{
				Objects: objects,
				Quiet:   aws.Bool(true),
			},
		})
		if err != nil {
			return deletedCount, fmt.Errorf("[MINIO] Failed to delete objects for folder %q: %w", folder, err)
		}
		if len(output.Errors) > 0 {
			return deletedCount, fmt.Errorf("[MINIO] Failed to delete %d objects for folder %q", len(output.Errors), folder)
		}

		deletedCount += uint32(len(objects))
	}

	return deletedCount, nil
}
