package objex

import "errors"

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
	ErrInvalidObjectName   = errors.New("INVALID_OBJECT_NAME")
)

type Bucket struct {
	Name         string
	CreationDate string
}

// TODO: write comments for each function
type Store interface {
	Setup() error
	SetBucket(bucketName string) (found bool, err error)
	SetRegion(region string) error
	CreateBucket(bucketName string) error
	DeleteBucket(bucketName string) error
	ListBuckets() ([]Bucket, error)
	CreateObject(fileName string, data []byte) error
	ReadObject(fileName string) ([]byte, error)
	UpdateObject(fileName string, data []byte) error
	DeleteObject(fileName string) error
	ListObjects(bucketName string) ([]string, error)
	Exists(fileName string) (bool, error)
	Metadata(fileName string) (map[string]string, error)
	CopyObject(fileSource, fileDestination string) error
	MoveObject(fileSource, fileDestination string) error
	CleanUp() error
	HealthCheck() error
}
