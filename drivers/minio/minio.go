package minio

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"strings"

	"github.com/brian-nunez/objex"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Config struct {
	Endpoint     string
	AccessKey    string
	SecretKey    string
	Token        string
	UseSSL       bool
	Region       string
	UsePathStyle bool
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
		Creds:  credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""),
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

func (s *Store) CreateObject(name string, data *os.File) error {
	if name == "" {
		return objex.ErrInvalidObjectName
	}

	bucketName := s.bucket
	fileName := name
	if bucketName == "" {
		log.Println("[Objex Minio] Warning: Empty bucket name, using full path for objects")
		paths := strings.Split(name, "/")
		bucketName = paths[0]
		fileName = strings.Join(paths[1:], "/")

		if bucketName == "" || fileName == "" {
			return objex.ErrInvalidObjectName
		}
	}

	fileInfo, err := data.Stat()
	if err != nil {
		return objex.ErrInvalidFile
	}

	_, err = s.client.PutObject(
		context.Background(),
		bucketName,
		fileName,
		data,
		fileInfo.Size(),
		minio.PutObjectOptions{
			ContentType: "application/octet-stream",
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

	bucketName := s.bucket
	fileName := name
	if bucketName == "" {
		log.Println("[Objex Minio] Warning: Empty bucket name, using full path for objects")
		paths := strings.Split(name, "/")
		bucketName = paths[0]
		fileName = strings.Join(paths[1:], "/")

		if bucketName == "" || fileName == "" {
			return nil, objex.ErrInvalidObjectName
		}
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

func (s *Store) UpdateObject(name string, data []byte) error {
	return nil
}

func (s *Store) DeleteObject(name string) error {
	if name == "" {
		return objex.ErrInvalidObjectName
	}

	bucketName := s.bucket
	fileName := name
	if bucketName == "" {
		log.Println("[Objex Minio] Warning: Empty bucket name, using full path for objects")
		paths := strings.Split(name, "/")
		bucketName = paths[0]
		fileName = strings.Join(paths[1:], "/")

		if bucketName == "" || fileName == "" {
			return objex.ErrInvalidObjectName
		}
	}

	err := s.client.RemoveObject(
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

// TODO: return bucket info instead of object name
func (s *Store) ListObjects(name string) ([]string, error) {
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

	objectNameList := make([]string, 0)
	for object := range objectChannel {
		if object.Err != nil {
			return nil, ToStandardError(object.Err)
		}

		if object.Key == "" {
			continue
		}

		objectNameList = append(objectNameList, object.Key)
	}

	return objectNameList, nil
}

func (s *Store) Exists(objectName string) (bool, error) {
	bucketName := s.bucket
	if bucketName == "" {
		log.Println("[Objex Minio] Warning: Empty bucket name, using full path for objects")
		paths := strings.Split(objectName, "/")
		bucketName = paths[0]
		objectName = strings.Join(paths[1:], "/")

		if bucketName == "" || objectName == "" {
			return false, objex.ErrInvalidObjectName
		}
	}

	_, err := s.client.StatObject(
		context.Background(),
		bucketName,
		objectName,
		minio.StatObjectOptions{},
	)

	if err != nil {
		standardErr := ToStandardError(err)
		if standardErr == objex.ErrObjectNotFound {
			return false, nil
		}

		return false, standardErr
	}

	return true, nil
}

func (s *Store) Metadata(objectName string) (*objex.ObjectMetaData, error) {
	bucketName := s.bucket
	if bucketName == "" {
		paths := strings.Split(objectName, "/")
		bucketName = paths[0]
		objectName = strings.Join(paths[1:], "/")

		if bucketName == "" || objectName == "" {
			return nil, objex.ErrInvalidObjectName
		}
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
	return nil
}

func (s *Store) MoveObject(src, dest string) error {
	return nil
}

func (s *Store) CleanUp() error {
	return nil
}
