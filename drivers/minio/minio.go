package minio

import (
	"context"
	"errors"
	"io"
	"log"
	"net/url"
	"time"

	"github.com/brian-nunez/objex"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var driverName = "minio"

func init() {
	objex.Register(driverName, func(config any) (objex.Store, error) {
		typed, ok := config.(Config)
		if !ok {
			return nil, objex.ErrClientInit
		}

		return NewStore(typed)
	})
}

type Config struct {
	Endpoint     string
	AccessKey    string
	SecretKey    string
	Token        string
	UseSSL       bool
	Region       string
	UsePathStyle bool
}

type minioClient interface {
	BucketExists(ctx context.Context, bucketName string) (bool, error)
	MakeBucket(ctx context.Context, bucketName string, options minio.MakeBucketOptions) error
	RemoveBucket(ctx context.Context, bucketName string) error
	ListBuckets(ctx context.Context) ([]minio.BucketInfo, error)
	PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (io.ReadCloser, error)
	RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error
	ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo
	StatObject(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error)
	CopyObject(ctx context.Context, dst minio.CopyDestOptions, src minio.CopySrcOptions) (minio.UploadInfo, error)
	PresignedGetObject(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error)
	PresignedPutObject(ctx context.Context, bucketName, objectName string, expires time.Duration) (*url.URL, error)
}

type minioClientWrapper struct {
	*minio.Client
}

func (w *minioClientWrapper) GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (io.ReadCloser, error) {
	return w.Client.GetObject(ctx, bucketName, objectName, opts)
}

var minioNew = func(endpoint string, opts *minio.Options) (minioClient, error) {
	c, err := minio.New(endpoint, opts)
	if err != nil {
		return nil, err
	}
	return &minioClientWrapper{c}, nil
}


func (c Config) DriverName() string {
	return driverName
}

type Store struct {
	config Config
	client minioClient
	bucket string
}

func ToStandardError(err error) error {
	if err == nil {
		return nil
	}

	code := minio.ToErrorResponse(err).Code

	if code == "" {
		return err
	}

	if code == "NoSuchBucket" {
		return objex.ErrBucketNotFound
	}

	if code == "NoSuchKey" {
		return objex.ErrObjectNotFound
	}

	if code == "AccessDenied" {
		return objex.ErrAccessDenied
	}

	if code == "Conflict" {
		return objex.ErrBucketNotEmpty
	}

	if code == "PreconditionFailed" {
		return objex.ErrPreconditionFailed
	}

	if code == "BucketAlreadyOwnedByYou" {
		return objex.ErrBucketAlreadyExists
	}

	return errors.New(code)
}

func NewStore(config Config) (*Store, error) {
	store := &Store{
		config: config,
	}

	err := store.HealthCheck(context.Background())
	if err != nil {
		return nil, err
	}

	minioClient, err := minioNew(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKey, config.SecretKey, config.Token),
		Secure: config.UseSSL,
	})
	if err != nil {
		return nil, objex.ErrClientInit
	}

	store.client = minioClient

	return store, nil
}

func (s *Store) Setup(ctx context.Context) error {
	return nil
}

func (s *Store) HealthCheck(ctx context.Context) error {
	if s.config.Endpoint == "" {
		return objex.ErrInvalidEndpoint
	}

	if s.config.AccessKey == "" {
		return objex.ErrInvalidAccessKey
	}

	if s.config.SecretKey == "" {
		return objex.ErrInvalidSecretKey
	}

	if s.config.Region == "" {
		log.Println("[Objex Minio] Warning: Region is not set, defaulting to 'us-east-1'")
		s.config.Region = "us-east-1"
	}

	return nil
}

func (s *Store) SetBucket(bucketName string) (found bool, err error) {
	if bucketName == "" {
		log.Println("[Objex Minio] Warning: Empty bucket name, using full path for objects")
		s.bucket = ""
		return false, nil
	}

	found, err = s.client.BucketExists(context.Background(), bucketName)
	if err != nil {
		standardErr := minio.ToErrorResponse(err)

		return found, standardErr
	}

	if !found {
		return found, objex.ErrBucketNotFound
	}

	s.bucket = bucketName

	return found, nil
}

func (s *Store) SetRegion(region string) error {
	if region == "" {
		log.Println("[Objex Minio] Warning: Region is not set, defaulting to 'us-east-1'")
		region = "us-east-1"
	}
	s.config.Region = region
	return nil
}

func (s *Store) CreateBucket(ctx context.Context, name string) error {
	if name == "" {
		return objex.ErrInvalidBucketName
	}

	err := s.client.MakeBucket(
		ctx,
		name,
		minio.MakeBucketOptions{
			Region: s.config.Region,
		},
	)

	standardErr := ToStandardError(err)
	if standardErr != nil {
		return standardErr
	}

	return nil
}

func (s *Store) DeleteBucket(ctx context.Context, name string) error {
	if name == "" {
		return objex.ErrInvalidBucketName
	}

	err := s.client.RemoveBucket(ctx, name)
	if err != nil {
		standardErr := ToStandardError(err)
		if standardErr == objex.ErrBucketNotFound {
			return nil
		}

		return standardErr
	}

	return nil
}

func (s *Store) ListBuckets(ctx context.Context) ([]objex.Bucket, error) {
	buckets, err := s.client.ListBuckets(ctx)
	if err != nil {
		return nil, ToStandardError(err)
	}

	var bucketItems []objex.Bucket
	for _, bucket := range buckets {
		bucketItems = append(bucketItems, objex.Bucket{
			Name:         bucket.Name,
			CreationDate: bucket.CreationDate.String(),
		})
	}

	return bucketItems, nil
}

func (s *Store) CreateObject(ctx context.Context, name string, data io.Reader, contentType string) (string, error) {
	if name == "" {
		return "", objex.ErrInvalidObjectName
	}

	bucketName, fileName, err := objex.SplitPath(s.bucket, name)
	if err != nil {
		return "", err
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	rd, size, err := objex.GetStreamSize(data)
	if err != nil {
		return "", objex.ErrPreconditionFailed
	}

	info, err := s.client.PutObject(
		ctx,
		bucketName,
		fileName,
		rd,
		size,
		minio.PutObjectOptions{
			ContentType: contentType,
		},
	)

	standardErr := ToStandardError(err)
	if standardErr != nil {
		return "", standardErr
	}

	return info.ETag, nil
}

func (s *Store) ReadObject(ctx context.Context, name string) (io.ReadCloser, error) {
	if name == "" {
		return nil, objex.ErrInvalidObjectName
	}

	bucketName, fileName, err := objex.SplitPath(s.bucket, name)
	if err != nil {
		return nil, err
	}

	object, err := s.client.GetObject(
		ctx,
		bucketName,
		fileName,
		minio.GetObjectOptions{},
	)
	if err != nil {
		standardErr := ToStandardError(err)
		return nil, standardErr
	}

	return object, nil
}

func (s *Store) ReadObjectRange(ctx context.Context, name string, offset, length int64) (io.ReadCloser, error) {
	if name == "" {
		return nil, objex.ErrInvalidObjectName
	}

	bucketName, fileName, err := objex.SplitPath(s.bucket, name)
	if err != nil {
		return nil, err
	}

	opts := minio.GetObjectOptions{}
	err = opts.SetRange(offset, offset+length-1)
	if err != nil {
		return nil, err
	}

	object, err := s.client.GetObject(
		ctx,
		bucketName,
		fileName,
		opts,
	)
	if err != nil {
		standardErr := ToStandardError(err)
		return nil, standardErr
	}

	return object, nil
}

func (s *Store) UpdateObject(ctx context.Context, name string, data io.Reader) (string, error) {
	exists, object, err := s.Exists(ctx, name)
	if err != nil {
		return "", err
	}

	if !exists {
		return "", objex.ErrObjectNotFound
	}

	return s.CreateObject(ctx, name, data, object.ContentType)
}

func (s *Store) DeleteObject(ctx context.Context, name string) error {
	if name == "" {
		return objex.ErrInvalidObjectName
	}

	bucketName, fileName, err := objex.SplitPath(s.bucket, name)
	if err != nil {
		return err
	}

	err = s.client.RemoveObject(
		ctx,
		bucketName,
		fileName,
		minio.RemoveObjectOptions{},
	)

	standardErr := ToStandardError(err)
	if standardErr != nil {
		return standardErr
	}

	return nil
}

func (s *Store) ListObjects(ctx context.Context, name string) ([]*objex.ObjectMetaData, error) {
	bucketName := s.bucket
	if bucketName == "" {
		bucketName = name
	}
	if bucketName == "" {
		return nil, objex.ErrInvalidBucketName
	}

	objectChannel := s.client.ListObjects(
		ctx,
		bucketName,
		minio.ListObjectsOptions{
			Recursive: true,
		},
	)

	var objects []*objex.ObjectMetaData
	for object := range objectChannel {
		if object.Err != nil {
			return nil, ToStandardError(object.Err)
		}

		if object.Key == "" {
			continue
		}

		objects = append(objects, &objex.ObjectMetaData{
			Key:          object.Key,
			Size:         object.Size,
			ContentType:  object.ContentType,
			ETag:         object.ETag,
			LastModified: object.LastModified.String(),
		})
	}

	return objects, nil
}

func (s *Store) Exists(ctx context.Context, name string) (bool, *objex.ObjectMetaData, error) {
	bucketName, name, err := objex.SplitPath(s.bucket, name)
	if err != nil {
		return false, nil, err
	}

	objectItem, err := s.client.StatObject(
		ctx,
		bucketName,
		name,
		minio.StatObjectOptions{},
	)

	if err != nil {
		standardErr := ToStandardError(err)
		if standardErr == objex.ErrObjectNotFound {
			return false, nil, nil
		}

		return false, nil, standardErr
	}

	metadata := &objex.ObjectMetaData{
		Key:          objectItem.Key,
		LastModified: objectItem.LastModified.String(),
		ETag:         objectItem.ETag,
		Size:         objectItem.Size,
		ContentType:  objectItem.ContentType,
	}

	return true, metadata, nil
}

func (s *Store) Metadata(ctx context.Context, objectName string) (*objex.ObjectMetaData, error) {
	bucketName, objectName, err := objex.SplitPath(s.bucket, objectName)
	if err != nil {
		return nil, err
	}

	objectItem, err := s.client.StatObject(
		ctx,
		bucketName,
		objectName,
		minio.StatObjectOptions{},
	)

	if err != nil {
		standardErr := ToStandardError(err)
		if standardErr == objex.ErrObjectNotFound {
			return nil, nil
		}

		return nil, standardErr
	}

	object := &objex.ObjectMetaData{
		Key:          objectItem.Key,
		LastModified: objectItem.LastModified.String(),
		ETag:         objectItem.ETag,
		Size:         objectItem.Size,
		ContentType:  objectItem.ContentType,
	}

	return object, nil
}

func (s *Store) CopyObject(ctx context.Context, src, dest string) error {
	if src == "" || dest == "" {
		return objex.ErrInvalidObjectName
	}

	srcBucket, srcKey, err := objex.SplitPath(s.bucket, src)
	if err != nil {
		return err
	}
	destBucket, destKey, err := objex.SplitPath(s.bucket, dest)
	if err != nil {
		return err
	}

	srcOpts := minio.CopySrcOptions{
		Bucket: srcBucket,
		Object: srcKey,
	}

	destOpts := minio.CopyDestOptions{
		Bucket: destBucket,
		Object: destKey,
	}

	_, err = s.client.CopyObject(ctx, destOpts, srcOpts)
	if err != nil {
		return ToStandardError(err)
	}

	return nil
}

func (s *Store) MoveObject(ctx context.Context, src, dest string) error {
	err := s.CopyObject(ctx, src, dest)
	if err != nil {
		return err
	}

	err = s.DeleteObject(ctx, src)
	if err != nil {
		return err
	}

	return nil
}

func (s *Store) PresignGet(ctx context.Context, name string, expiration time.Duration) (string, error) {
	bucketName, fileName, err := objex.SplitPath(s.bucket, name)
	if err != nil {
		return "", err
	}

	u, err := s.client.PresignedGetObject(ctx, bucketName, fileName, expiration, url.Values{})
	if err != nil {
		return "", ToStandardError(err)
	}

	return u.String(), nil
}

func (s *Store) PresignPut(ctx context.Context, name string, expiration time.Duration) (string, error) {
	bucketName, fileName, err := objex.SplitPath(s.bucket, name)
	if err != nil {
		return "", err
	}

	u, err := s.client.PresignedPutObject(ctx, bucketName, fileName, expiration)
	if err != nil {
		return "", ToStandardError(err)
	}

	return u.String(), nil
}

func (s *Store) CleanUp() error {
	return nil
}
