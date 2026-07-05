package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// s3api is the subset of the AWS S3 SDK client used by S3Client.
type s3api interface {
	CreateBucket(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error)
	DeleteBucket(ctx context.Context, params *s3.DeleteBucketInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketOutput, error)
	HeadBucket(ctx context.Context, params *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error)
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	DeleteObjects(ctx context.Context, params *s3.DeleteObjectsInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error)
}

// S3Client wraps AWS S3 API operations for managing developer buckets.
type S3Client struct {
	client s3api
	region string
}

// NewS3Client creates a new S3 client configured with the specified AWS profile and region.
func NewS3Client(ctx context.Context, profile, region string) (*S3Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for S3: %w", err)
	}

	return &S3Client{
		client: s3.NewFromConfig(cfg),
		region: region,
	}, nil
}

// BucketName returns the canonical bucket name for a developer.
func BucketName(developerName string) string {
	return fmt.Sprintf("clouddesktop-%s", developerName)
}

// CreateBucket creates an S3 bucket for the developer. Returns nil if the bucket already exists.
func (c *S3Client) CreateBucket(ctx context.Context, bucketName string) error {
	input := &s3.CreateBucketInput{
		Bucket: &bucketName,
	}

	// us-east-1 does not accept a LocationConstraint
	if c.region != "us-east-1" {
		input.CreateBucketConfiguration = &s3types.CreateBucketConfiguration{
			LocationConstraint: s3types.BucketLocationConstraint(c.region),
		}
	}

	_, err := c.client.CreateBucket(ctx, input)
	if err != nil {
		if isBucketAlreadyOwnedError(err) {
			return nil
		}
		return fmt.Errorf("failed to create bucket %s: %w", bucketName, err)
	}

	return nil
}

// BucketExists checks if the bucket exists and is accessible.
func (c *S3Client) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	_, err := c.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: &bucketName,
	})
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check bucket %s: %w", bucketName, err)
	}
	return true, nil
}

// EmptyBucket deletes all objects in the bucket. Required before DeleteBucket.
func (c *S3Client) EmptyBucket(ctx context.Context, bucketName string) error {
	paginator := true
	var continuationToken *string

	for paginator {
		output, err := c.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            &bucketName,
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return fmt.Errorf("failed to list objects in bucket %s: %w", bucketName, err)
		}

		if len(output.Contents) == 0 {
			break
		}

		objects := make([]s3types.ObjectIdentifier, len(output.Contents))
		for i, obj := range output.Contents {
			objects[i] = s3types.ObjectIdentifier{Key: obj.Key}
		}

		_, err = c.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: &bucketName,
			Delete: &s3types.Delete{
				Objects: objects,
				Quiet:   aws.Bool(true),
			},
		})
		if err != nil {
			return fmt.Errorf("failed to delete objects from bucket %s: %w", bucketName, err)
		}

		if output.IsTruncated != nil && *output.IsTruncated {
			continuationToken = output.NextContinuationToken
		} else {
			paginator = false
		}
	}

	return nil
}

// DeleteBucket deletes an S3 bucket. The bucket must be empty first.
func (c *S3Client) DeleteBucket(ctx context.Context, bucketName string) error {
	_, err := c.client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: &bucketName,
	})
	if err != nil {
		if isNotFoundError(err) {
			return nil
		}
		return fmt.Errorf("failed to delete bucket %s: %w", bucketName, err)
	}
	return nil
}

// isBucketAlreadyOwnedError checks if the error indicates the bucket already exists
// and is owned by the caller.
func isBucketAlreadyOwnedError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "BucketAlreadyOwnedByYou") || strings.Contains(msg, "BucketAlreadyExists")
}
