package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type S3Store struct {
	client *s3.Client
	bucket string
}

// NewS3Store talks to any S3 compatible server. endpoint is "" for real AWS,
// or e.g. http://localhost:9000 for MinIO. Credentials come from the usual
// AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY env vars.
func NewS3Store(bucket string, endpoint string) (*S3Store, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion("us-east-1"))
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true // MinIO wants localhost:9000/bucket/key, not bucket.localhost
		}
	})
	return &S3Store{client: client, bucket: bucket}, nil
}

// Create never overwrites: If-None-Match: * makes S3 refuse when the key exists.
func (s *S3Store) Create(name string, data []byte) error {
	_, err := s.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(name),
		Body:        bytes.NewReader(data),
		IfNoneMatch: aws.String("*"),
	})
	if err != nil {
		// 412 PreconditionFailed: the key already exists. 409 means another
		// write to the same key was in flight, we lost that race too.
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			code := apiErr.ErrorCode()
			if code == "PreconditionFailed" || code == "ConditionalRequestConflict" {
				return ErrExists
			}
		}
		return err
	}
	return nil
}

func (s *S3Store) Read(name string) ([]byte, error) {
	out, err := s.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(name),
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

// ReadAt reads exactly n bytes starting at off, with a Range header.
func (s *S3Store) ReadAt(name string, off int64, n int) ([]byte, error) {
	if n == 0 {
		return []byte{}, nil
	}
	out, err := s.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(name),
		Range:  aws.String(fmt.Sprintf("bytes=%d-%d", off, off+int64(n)-1)), // both ends are inclusive
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()

	b, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, err
	}
	if len(b) != n {
		return nil, fmt.Errorf("%s: asked for %d bytes at %d, got %d", name, n, off, len(b))
	}
	return b, nil
}

// List returns every key under the prefix, sorted by name (S3 already returns them that way).
func (s *S3Store) List(p Prefix) ([]string, error) {
	var names []string
	pages := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(string(p)),
	})
	for pages.HasMorePages() {
		page, err := pages.NextPage(context.Background())
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			names = append(names, *obj.Key)
		}
	}
	return names, nil
}
