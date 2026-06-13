package aws

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/brian-nunez/objex"
)

const driverName = "aws"

type s3Client interface {
	ListBuckets(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error)
	HeadBucket(ctx context.Context, params *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error)
	CreateBucket(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error)
	DeleteBucket(ctx context.Context, params *s3.DeleteBucketInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	CopyObject(ctx context.Context, params *s3.CopyObjectInput, optFns ...func(*s3.Options)) (*s3.CopyObjectOutput, error)
}

type s3PresignClient interface {
	PresignGetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
	PresignPutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

type s3Uploader interface {
	Upload(ctx context.Context, input *s3.PutObjectInput, opts ...func(*manager.Uploader)) (*manager.UploadOutput, error)
}

var loadDefaultConfig = config.LoadDefaultConfig

func init() {
	objex.Register(driverName, func(cfg any) (objex.Store, error) {
		typed, ok := cfg.(Config)
		if !ok {
			return nil, objex.ErrClientInit
		}

		return NewStore(typed)
	})
}

type Config struct {
	Region       string
	Bucket       string
	Endpoint     string
	AccessKey    string
	SecretKey    string
	Token        string
	UseSSL       bool
	UsePathStyle bool
}

func (c Config) DriverName() string {
	return driverName
}

type Store struct {
	client        s3Client
	presignClient s3PresignClient
	uploader      s3Uploader
	bucket        string
	region        string
}

func NewStore(cfg Config) (*Store, error) {
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}

	var awsCfg aws.Config
	var err error

	customCreds := aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(
		cfg.AccessKey, cfg.SecretKey, cfg.Token,
	))

	if cfg.Endpoint != "" {
		awsCfg, err = loadDefaultConfig(context.TODO(),
			config.WithRegion(cfg.Region),
			config.WithCredentialsProvider(customCreds),
			config.WithEndpointResolverWithOptions(
				aws.EndpointResolverWithOptionsFunc(func(service, region string, _ ...interface{}) (aws.Endpoint, error) {
					return aws.Endpoint{
						URL:               objex.Scheme(cfg.UseSSL) + "://" + cfg.Endpoint,
						SigningRegion:     cfg.Region,
						HostnameImmutable: true,
					}, nil
				}),
			),
		)
	} else {
		awsCfg, err = loadDefaultConfig(context.TODO(),
			config.WithRegion(cfg.Region),
			config.WithCredentialsProvider(customCreds),
		)
	}

	if err != nil {
		return nil, objex.ErrClientInit
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.UsePathStyle
	})

	return &Store{
		client:        client,
		presignClient: s3.NewPresignClient(client),
		uploader:      manager.NewUploader(client),
		bucket:        cfg.Bucket,
		region:        cfg.Region,
	}, nil
}

func (s *Store) DriverName() string {
	return driverName
}

func (s *Store) Setup(ctx context.Context) error { return nil }

func (s *Store) HealthCheck(ctx context.Context) error {
	_, err := s.client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return objex.ErrClientInit
	}

	return nil
}

func (s *Store) SetBucket(bucketName string) (bool, error) {
	s.bucket = bucketName
	_, err := s.client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return false, objex.ErrBucketNotFound
	}
	return true, nil
}

func (s *Store) SetRegion(region string) error {
	s.region = region
	return nil
}

func (s *Store) CreateBucket(ctx context.Context, name string) error {
	if name == "" {
		return objex.ErrInvalidBucketName
	}

	_, err := s.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(name),
	})
	if err != nil {
		return objex.ErrBucketAlreadyExists
	}

	return nil
}

func (s *Store) DeleteBucket(ctx context.Context, name string) error {
	_, err := s.client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(name),
	})
	return err
}

func (s *Store) ListBuckets(ctx context.Context) ([]objex.Bucket, error) {
	out, err := s.client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}

	var buckets []objex.Bucket
	for _, b := range out.Buckets {
		buckets = append(buckets, objex.Bucket{
			Name:         *b.Name,
			CreationDate: b.CreationDate.Format(time.RFC3339),
		})
	}

	return buckets, nil
}

func (s *Store) CreateObject(ctx context.Context, name string, data io.Reader, contentType string) (string, error) {
	bucket, key, err := objex.SplitPath(s.bucket, name)
	if err != nil {
		return "", err
	}

	out, err := s.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        data,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", err
	}

	return aws.ToString(out.ETag), nil
}

func (s *Store) ReadObject(ctx context.Context, name string) (io.ReadCloser, error) {
	bucket, key, err := objex.SplitPath(s.bucket, name)
	if err != nil {
		return nil, err
	}

	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}

	return out.Body, nil
}

func (s *Store) ReadObjectRange(ctx context.Context, name string, offset, length int64) (io.ReadCloser, error) {
	bucket, key, err := objex.SplitPath(s.bucket, name)
	if err != nil {
		return nil, err
	}

	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Range:  aws.String(fmt.Sprintf("bytes=%d-%d", offset, offset+length-1)),
	})
	if err != nil {
		return nil, err
	}

	return out.Body, nil
}

func (s *Store) UpdateObject(ctx context.Context, name string, data io.Reader) (string, error) {
	exists, meta, err := s.Exists(ctx, name)
	if err != nil || !exists {
		return "", objex.ErrObjectNotFound
	}
	return s.CreateObject(ctx, name, data, meta.ContentType)
}

func (s *Store) DeleteObject(ctx context.Context, name string) error {
	bucket, key, err := objex.SplitPath(s.bucket, name)
	if err != nil {
		return err
	}

	_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}

func (s *Store) ListObjects(ctx context.Context, bucketName string) ([]*objex.ObjectMetaData, error) {
	if s.bucket == "" {
		s.bucket = bucketName
	}

	out, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
	})
	if err != nil {
		return nil, err
	}

	var items []*objex.ObjectMetaData
	for _, obj := range out.Contents {
		items = append(items, &objex.ObjectMetaData{
			Key:          *obj.Key,
			Size:         *obj.Size,
			LastModified: obj.LastModified.Format(time.RFC3339),
			ETag:         aws.ToString(obj.ETag),
			ContentType:  "application/octet-stream",
		})
	}
	return items, nil
}

func (s *Store) Exists(ctx context.Context, name string) (bool, *objex.ObjectMetaData, error) {
	bucket, key, err := objex.SplitPath(s.bucket, name)
	if err != nil {
		return false, nil, err
	}

	head, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var nf *types.NotFound
		if errors.As(err, &nf) {
			return false, nil, nil
		}
		return false, nil, err
	}

	meta := &objex.ObjectMetaData{
		Key:          key,
		Size:         *head.ContentLength,
		ContentType:  aws.ToString(head.ContentType),
		LastModified: head.LastModified.Format(time.RFC3339),
		ETag:         aws.ToString(head.ETag),
	}
	return true, meta, nil
}

func (s *Store) Metadata(ctx context.Context, name string) (*objex.ObjectMetaData, error) {
	ok, meta, err := s.Exists(ctx, name)
	if err != nil || !ok {
		return nil, err
	}
	return meta, nil
}

func (s *Store) CopyObject(ctx context.Context, src, dest string) error {
	srcBucket, srcKey, err := objex.SplitPath(s.bucket, src)
	if err != nil {
		return err
	}
	destBucket, destKey, err := objex.SplitPath(s.bucket, dest)
	if err != nil {
		return err
	}

	source := srcBucket + "/" + srcKey
	_, err = s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(destBucket),
		Key:        aws.String(destKey),
		CopySource: aws.String(source),
	})
	return err
}

func (s *Store) MoveObject(ctx context.Context, src, dest string) error {
	err := s.CopyObject(ctx, src, dest)
	if err != nil {
		return err
	}
	return s.DeleteObject(ctx, src)
}

func (s *Store) PresignGet(ctx context.Context, name string, expiration time.Duration) (string, error) {
	bucket, key, err := objex.SplitPath(s.bucket, name)
	if err != nil {
		return "", err
	}

	out, err := s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiration))
	if err != nil {
		return "", err
	}

	return out.URL, nil
}

func (s *Store) PresignPut(ctx context.Context, name string, expiration time.Duration) (string, error) {
	bucket, key, err := objex.SplitPath(s.bucket, name)
	if err != nil {
		return "", err
	}

	out, err := s.presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiration))
	if err != nil {
		return "", err
	}

	return out.URL, nil
}

func (s *Store) CleanUp() error {
	return nil
}
