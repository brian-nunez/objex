package minio

import (
	"context"
	"errors"
	"log"

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

var (
	ErrInvalidEndpoint     = errors.New("INVALID_ENDPOINT")
	ErrInvalidAccessKey    = errors.New("INVALID_ACCESS_KEY")
	ErrInvalidSecretKey    = errors.New("INVALID_SECRET_KEY")
	ErrClientInit          = errors.New("CLIENT_INIT_FAILED")
	ErrBucketNotFound      = errors.New("BUCKET_NOT_FOUND")
	ErrInvalidBucketName   = errors.New("INVALID_BUCKET_NAME")
	ErrObjectNotFound      = errors.New("OBJECT_NOT_FOUND")
	ErrAccessDenied        = errors.New("ACCESS_DENIED")
	ErrBucketNotEmpty      = errors.New("BUCKET_NOT_EMPTY")
	ErrPreconditionFailed  = errors.New("PRECONDITION_FAILED")
	ErrBucketAlreadyExists = errors.New("BUCKET_ALREADY_EXISTS")
)

func ToStandardError(err error) error {
	if err == nil {
		return nil
	}

	code := minio.ToErrorResponse(err).Code

	if code == "" {
		return nil
	}

	if code == "NoSuchBucket" {
		return ErrBucketNotFound
	}

	if code == "NoSuchKey" {
		return ErrObjectNotFound
	}

	if code == "AccessDenied" {
		return ErrAccessDenied
	}

	if code == "Conflict" {
		return ErrBucketNotEmpty
	}

	if code == "PreconditionFailed" {
		return ErrPreconditionFailed
	}

	if code == "BucketAlreadyOwnedByYou" {
		return ErrBucketAlreadyExists
	}

	return errors.New(code)
}

func NewStore(config Config) (*Store, error) {
	if config.Endpoint == "" {
		return nil, ErrInvalidEndpoint
	}

	if config.AccessKey == "" {
		return nil, ErrInvalidAccessKey
	}

	if config.SecretKey == "" {
		return nil, ErrInvalidSecretKey
	}

	if config.Region == "" {
		log.Println("[Objex Minio] Warning: Region is not set, defaulting to 'us-east-1'")
		config.Region = "us-east-1"
	}

	if !config.UseSSL {
		log.Println("[Objex Minio] Warning: Using HTTP instead of HTTPS")
	}

	minioClient, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""),
		Secure: config.UseSSL,
	})
	if err != nil {
		return nil, ErrClientInit
	}

	store := &Store{
		config: config,
		client: minioClient,
	}

	return store, nil
}

func (s *Store) Setup() error {
	return nil
}

func (s *Store) HealthCheck() error {
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
		return found, ErrBucketNotFound
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
		return ErrInvalidBucketName
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
		return ErrInvalidBucketName
	}

	err := s.client.RemoveBucket(context.Background(), name)
	if err != nil {
		standardErr := ToStandardError(err)
		if standardErr == ErrBucketNotFound {
			return nil
		}

		return standardErr
	}

	return nil
}

func (s *Store) ListBuckets() ([]string, error) {
	return []string{}, nil
}

func (s *Store) CreateObject(name string, data []byte) error {
	return nil
}

func (s *Store) ReadObject(name string) ([]byte, error) {
	return nil, nil
}

func (s *Store) UpdateObject(name string, data []byte) error {
	return nil
}

func (s *Store) DeleteObject(name string) error {
	return nil
}

func (s *Store) ListObjects(name string) ([]string, error) {
	return []string{}, nil
}

func (s *Store) Exists(name string) (bool, error) {
	return false, nil
}

func (s *Store) Metadata(name string) (map[string]string, error) {
	return map[string]string{}, nil
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
