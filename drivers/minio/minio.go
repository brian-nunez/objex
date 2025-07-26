package minio

import (
	"context"
	"errors"
	"io"
	"log"
	"strings"

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

func (c Config) DriverName() string {
	return driverName
}

type Store struct {
	config Config
	client *minio.Client
	bucket string
}

func ToStandardError(err error) error {
	if err == nil {
		return nil
	}

	code := minio.ToErrorResponse(err).Code

	if code == "" {
		return nil
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

	err := store.HealthCheck()
	if err != nil {
		return nil, err
	}

	minioClient, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKey, config.SecretKey, config.Token),
		Secure: config.UseSSL,
	})
	if err != nil {
		return nil, objex.ErrClientInit
	}

	store.client = minioClient

	return store, nil
}

func (s *Store) Setup() error {
	return nil
}

func (s *Store) HealthCheck() error {
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

	if !s.config.UseSSL {
		log.Println("[Objex Minio] Warning: Using HTTP instead of HTTPS")
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

func (s *Store) CreateBucket(name string) error {
	if name == "" {
		return objex.ErrInvalidBucketName
	}

	err := s.client.MakeBucket(
		context.Background(),
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

func (s *Store) DeleteBucket(name string) error {
	if name == "" {
		return objex.ErrInvalidBucketName
	}

	err := s.client.RemoveBucket(context.Background(), name)
	if err != nil {
		standardErr := ToStandardError(err)
		if standardErr == objex.ErrBucketNotFound {
			return nil
		}

		return standardErr
	}

	return nil
}

func (s *Store) ListBuckets() ([]objex.Bucket, error) {
	buckets, err := s.client.ListBuckets(context.Background())
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

func (s *Store) CreateObject(name string, data io.Reader, contentType string) error {
	if name == "" {
		return objex.ErrInvalidObjectName
	}

	bucketName, fileName, err := objex.SplitPath(s.bucket, name)
	if err != nil {
		return err
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, size, err := objex.GetStreamSize(data)
	if err != nil {
		return objex.ErrPreconditionFailed
	}

	_, err = s.client.PutObject(
		context.Background(),
		bucketName,
		fileName,
		data,
		size,
		minio.PutObjectOptions{
			ContentType: contentType,
		},
	)

	standardErr := ToStandardError(err)
	if standardErr != nil {
		return standardErr
	}

	return nil
}

func (s *Store) ReadObject(name string) ([]byte, error) {
	if name == "" {
		return nil, objex.ErrInvalidObjectName
	}

	bucketName, fileName, err := objex.SplitPath(s.bucket, name)
	if err != nil {
		return nil, err
	}

	object, err := s.client.GetObject(
		context.Background(),
		bucketName,
		fileName,
		minio.GetObjectOptions{},
	)
	if err != nil {
		standardErr := ToStandardError(err)
		if standardErr == objex.ErrObjectNotFound {
			return nil, nil
		}

		return nil, standardErr
	}
	defer object.Close()

	objectData, err := io.ReadAll(object)
	if err != nil {
		return nil, err
	}

	return objectData, nil
}

func (s *Store) UpdateObject(name string, data io.Reader) error {
	exists, object, err := s.Exists(name)
	if err != nil {
		return err
	}

	if !exists {
		return objex.ErrObjectNotFound
	}

	return s.CreateObject(name, data, object.ContentType)
}

func (s *Store) DeleteObject(name string) error {
	if name == "" {
		return objex.ErrInvalidObjectName
	}

	bucketName, fileName, err := objex.SplitPath(s.bucket, name)
	if err != nil {
		return err
	}

	err = s.client.RemoveObject(
		context.Background(),
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

func (s *Store) ListObjects(name string) ([]*objex.ObjectMetaData, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

func (s *Store) Exists(name string) (bool, *objex.ObjectMetaData, error) {
	bucketName, name, err := objex.SplitPath(s.bucket, name)
	if err != nil {
		return false, nil, err
	}

	objectItem, err := s.client.StatObject(
		context.Background(),
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

func (s *Store) Metadata(objectName string) (*objex.ObjectMetaData, error) {
	bucketName, objectName, err := objex.SplitPath(s.bucket, objectName)
	if err != nil {
		return nil, err
	}

	objectItem, err := s.client.StatObject(
		context.Background(),
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

func (s *Store) CopyObject(src, dest string) error {
	if src == "" || dest == "" {
		return objex.ErrInvalidObjectName
	}

	srcBucket := s.bucket
	srcKey := src
	destBucket := s.bucket
	destKey := dest

	if srcBucket == "" {
		paths := strings.SplitN(src, "/", 2)
		if len(paths) < 2 {
			return objex.ErrInvalidObjectName
		}
		srcBucket = paths[0]
		srcKey = paths[1]
	}

	if destBucket == "" {
		paths := strings.SplitN(dest, "/", 2)
		if len(paths) < 2 {
			return objex.ErrInvalidObjectName
		}
		destBucket = paths[0]
		destKey = paths[1]
	}

	srcOpts := minio.CopySrcOptions{
		Bucket: srcBucket,
		Object: srcKey,
	}

	destOpts := minio.CopyDestOptions{
		Bucket: destBucket,
		Object: destKey,
	}

	_, err := s.client.CopyObject(context.Background(), destOpts, srcOpts)
	if err != nil {
		return ToStandardError(err)
	}

	return nil
}

func (s *Store) MoveObject(src, dest string) error {
	err := s.CopyObject(src, dest)
	if err != nil {
		return err
	}

	err = s.DeleteObject(src)
	if err != nil {
		return err
	}

	return nil
}

func (s *Store) CleanUp() error {
	log.Println("[Objex Minio] CleanUp called — no action needed")
	return nil
}
