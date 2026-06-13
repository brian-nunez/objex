package objex

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	ErrUnknownDriver       = errors.New("UNKNOWN_DRIVER")
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
	ErrInvalidObjectName   = errors.New("INVALID_OBJECT_NAME")
	ErrInvalidFile         = errors.New("INVALID_FILE")
)

type Bucket struct {
	Name         string
	CreationDate string
}

type ObjectMetaData struct {
	Key          string
	Size         int64
	ContentType  string
	ETag         string
	LastModified string
}

// Store is the interface for object storage operations.
type Store interface {
	Setup(ctx context.Context) error
	SetBucket(bucketName string) (found bool, err error)
	SetRegion(region string) error
	CreateBucket(ctx context.Context, bucketName string) error
	DeleteBucket(ctx context.Context, bucketName string) error
	ListBuckets(ctx context.Context) ([]Bucket, error)
	CreateObject(ctx context.Context, objectName string, data io.Reader, contentType string) (string, error)
	ReadObject(ctx context.Context, fileName string) (io.ReadCloser, error)
	ReadObjectRange(ctx context.Context, fileName string, offset, length int64) (io.ReadCloser, error)
	UpdateObject(ctx context.Context, fileName string, data io.Reader) (string, error)
	DeleteObject(ctx context.Context, fileName string) error
	ListObjects(ctx context.Context, bucketName string) ([]*ObjectMetaData, error)
	Exists(ctx context.Context, fileName string) (bool, *ObjectMetaData, error)
	Metadata(ctx context.Context, fileName string) (*ObjectMetaData, error)
	CopyObject(ctx context.Context, fileSource, fileDestination string) error
	MoveObject(ctx context.Context, fileSource, fileDestination string) error
	PresignGet(ctx context.Context, name string, expiration time.Duration) (string, error)
	PresignPut(ctx context.Context, name string, expiration time.Duration) (string, error)
	CleanUp() error
	HealthCheck(ctx context.Context) error
}
