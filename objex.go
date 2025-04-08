package objex

// TODO: write comments for each function
type Store interface {
	Setup() error
	SetBucket(bucketName string) (found bool, err error)
	SetRegion(region string) error
	CreateBucket(bucketName string) error
	DeleteBucket(bucketName string) error
	ListBuckets() ([]string, error)
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
