package aws

import (
	"context"
	"errors"
	"io"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/brian-nunez/objex"
)

const driverName = "aws"

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
	client     *s3.Client
	uploader   *manager.Uploader
	downloader *manager.Downloader
	bucket     string
	region     string
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
		awsCfg, err = config.LoadDefaultConfig(context.TODO(),
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
		awsCfg, err = config.LoadDefaultConfig(context.TODO(),
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
		client:     client,
		uploader:   manager.NewUploader(client),
		downloader: manager.NewDownloader(client),
		bucket:     cfg.Bucket,
		region:     cfg.Region,
	}, nil
}

func (s *Store) DriverName() string {
	return driverName
}

func (s *Store) Setup() error { return nil }

func (s *Store) HealthCheck() error {
	_, err := s.client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
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

func (s *Store) CreateBucket(name string) error {
	if name == "" {
		return objex.ErrInvalidBucketName
	}

	_, err := s.client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
		Bucket: aws.String(name),
	})
	if err != nil {
		return objex.ErrBucketAlreadyExists
	}

	return nil
}

func (s *Store) DeleteBucket(name string) error {
	_, err := s.client.DeleteBucket(context.TODO(), &s3.DeleteBucketInput{
		Bucket: aws.String(name),
	})
	return err
}

func (s *Store) ListBuckets() ([]objex.Bucket, error) {
	out, err := s.client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
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

func (s *Store) CreateObject(name string, data io.Reader, contentType string) error {
	bucket, key, err := objex.SplitPath(s.bucket, name)
	if err != nil {
		return err
	}

	rd, _, err := objex.GetStreamSize(data)
	if err != nil {
		return objex.ErrPreconditionFailed
	}

	_, err = s.uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        rd,
		ContentType: aws.String(contentType),
	})
	return err
}

func (s *Store) ReadObject(name string) ([]byte, error) {
	bucket, key, err := objex.SplitPath(s.bucket, name)
	if err != nil {
		return nil, err
	}

	buf := manager.NewWriteAtBuffer([]byte{})
	_, err = s.downloader.Download(context.TODO(), buf, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (s *Store) UpdateObject(name string, data io.Reader) error {
	exists, meta, err := s.Exists(name)
	if err != nil || !exists {
		return objex.ErrObjectNotFound
	}
	return s.CreateObject(name, data, meta.ContentType)
}

func (s *Store) DeleteObject(name string) error {
	bucket, key, err := objex.SplitPath(s.bucket, name)
	if err != nil {
		return err
	}

	_, err = s.client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}

func (s *Store) ListObjects(bucketName string) ([]*objex.ObjectMetaData, error) {
	if s.bucket == "" {
		s.bucket = bucketName
	}

	out, err := s.client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
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
			ETag:         *obj.ETag,
			ContentType:  "application/octet-stream", // AWS S3 doesn't return this in List
		})
	}
	return items, nil
}

func (s *Store) Exists(name string) (bool, *objex.ObjectMetaData, error) {
	bucket, key, err := objex.SplitPath(s.bucket, name)
	if err != nil {
		return false, nil, err
	}

	head, err := s.client.HeadObject(context.TODO(), &s3.HeadObjectInput{
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

func (s *Store) Metadata(name string) (*objex.ObjectMetaData, error) {
	ok, meta, err := s.Exists(name)
	if err != nil || !ok {
		return nil, err
	}
	return meta, nil
}

func (s *Store) CopyObject(src, dest string) error {
	srcBucket, srcKey, err := objex.SplitPath(s.bucket, src)
	if err != nil {
		return err
	}
	destBucket, destKey, err := objex.SplitPath(s.bucket, dest)
	if err != nil {
		return err
	}

	source := srcBucket + "/" + srcKey
	_, err = s.client.CopyObject(context.TODO(), &s3.CopyObjectInput{
		Bucket:     aws.String(destBucket),
		Key:        aws.String(destKey),
		CopySource: aws.String(source),
	})
	return err
}

func (s *Store) MoveObject(src, dest string) error {
	err := s.CopyObject(src, dest)
	if err != nil {
		return err
	}
	return s.DeleteObject(src)
}

func (s *Store) CleanUp() error {
	log.Println("[Objex AWS] CleanUp called — no action needed")
	return nil
}
