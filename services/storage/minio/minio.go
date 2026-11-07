package minio

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

var (
	// ErrInvalidKey: folder hoặc tên file rỗng nên không dựng được object key.
	ErrInvalidKey = errors.New("[MINIO] Object key cannot be empty")
	// ErrObjectNotFound: object không tồn tại trong bucket.
	ErrObjectNotFound = errors.New("[MINIO] Object not found")
)

func (c *Client) GeneratePresignedUploadURL(ctx context.Context, folders []BucketFolder, fileName string, contentType string, internal bool) (*string, error) {
	if err := ValidateUploadFolders(folders); err != nil {
		return nil, err
	}
	objectKey := BuildObjectKey(folders, fileName)
	if objectKey == "" {
		return nil, ErrInvalidKey
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	presignClient := c.PresignClient
	if internal {
		presignClient = c.InternalPresignClient
	}

	request, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
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

func (c *Client) GetPublicURL(folders []BucketFolder, fileName string) (*string, error) {
	objectKey := BuildObjectKey(folders, fileName)
	if objectKey == "" {
		return nil, ErrInvalidKey
	}
	if strings.TrimSpace(c.PublicEndpoint) == "" {
		return nil, errors.New("[MINIO] R2_PUBLIC_ENDPOINT is required to build a viewable public URL")
	}

	url := fmt.Sprintf("%s/%s", strings.TrimRight(c.PublicEndpoint, "/"), objectKey)
	return &url, nil
}

func (c *Client) GetObjectMetadata(ctx context.Context, folders []BucketFolder, fileName string) (int64, string, error) {
	objectKey := BuildObjectKey(folders, fileName)
	if objectKey == "" {
		return 0, "", ErrInvalidKey
	}
	output, err := c.Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(objectKey),
	})
	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return 0, "", ErrObjectNotFound
	}
	if err != nil {
		return 0, "", err
	}
	return aws.ToInt64(output.ContentLength), aws.ToString(output.ContentType), nil
}

func (c *Client) DeleteObject(ctx context.Context, folders []BucketFolder, fileName string) error {
	objectKey := BuildObjectKey(folders, fileName)
	if objectKey == "" {
		return ErrInvalidKey
	}
	_, err := c.Client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(c.Bucket), Key: aws.String(objectKey)})
	return err
}

func (c *Client) DeleteFolder(ctx context.Context, folder string) (uint32, error) {
	prefix := strings.Trim(folder, "/")
	if prefix == "" {
		return 0, ErrInvalidKey
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
