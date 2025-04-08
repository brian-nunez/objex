package objex

// TODO: write comments for each function
type Store interface {
	Setup() error
	SetBucket(name string) error
	CreateBucket(name string) error
	DeleteBucket(name string) error
	CreateObject(name string, data []byte) error
	ReadObject(name string) ([]byte, error)
	UpdateObject(name string, data []byte) error
	DeleteObject(name string) error
	Exists(name string) (bool, error)
	Metadata(name string) (map[string]string, error)
	CopyObject(src, dest string) error
	MoveObject(src, dest string) error
	CleanUp() error
}
