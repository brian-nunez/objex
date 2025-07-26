package objex

import (
	"errors"
	"io"
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

// TODO: write comments for each function
type Store interface {
	Setup() error
	SetBucket(bucketName string) (found bool, err error)
	SetRegion(region string) error
	CreateBucket(bucketName string) error
	DeleteBucket(bucketName string) error
	ListBuckets() ([]Bucket, error)
	CreateObject(objectName string, data io.Reader, contentType string) error
	ReadObject(fileName string) ([]byte, error)
	UpdateObject(fileName string, data io.Reader) error
	DeleteObject(fileName string) error
	ListObjects(bucketName string) ([]*ObjectMetaData, error)
	Exists(fileName string) (bool, *ObjectMetaData, error)
	Metadata(fileName string) (*ObjectMetaData, error)
	CopyObject(fileSource, fileDestination string) error
	MoveObject(fileSource, fileDestination string) error
	CleanUp() error
	HealthCheck() error
}
