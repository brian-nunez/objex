# Object Storage Extendable Interface for Go

`objex` is a pluggable abstraction layer for interacting with object storage services like AWS S3, MinIO, Cloudflare R2, and other S3-compatible providers.

It provides a unified interface to:

* Upload, download, and manage objects
* List and inspect buckets
* Swap object store implementations at runtime
* Write tools that don’t care what backend is used

## 🔧 Supported Drivers

| Driver  | Package                                      | Use Case                            |
| :------ | :------------------------------------------- | :---------------------------------- |
| `aws`   | `github.com/brian-nunez/objex/drivers/aws`   | AWS S3 or any S3-compatible backend |
| `minio` | `github.com/brian-nunez/objex/drivers/minio` | MinIO (self-hosted, Docker, etc.)   |

Each driver registers itself via `init()` and can be instantiated through a single call to `objex.New(config)`.


## Why Use objex?

* No need to learn each storage SDK (you probably should, but...)
* Pluggable and testable interface
* Easily swap from S3 to MinIO or R2 with zero changes to your application logic
* Works seamlessly with both file-based and streamed data

## Quick Start (Using AWS)

1. Import the package and the driver

```go
import (
	"github.com/brian-nunez/objex"
	"github.com/brian-nunez/objex/drivers/aws" // registers "aws" driver
)
```

2. Create a new store

```go
store, err := objex.New(aws.Config{
	Region:    "us-east-1",
	Bucket:    "my-app-assets",
	AccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
	SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
	Token:     "", // Optional, used for temporary sessions
})
```

3. Use it

```go
// Upload a file
f, _ := os.Open("cat.png")
defer f.Close()

err = store.CreateObject("images/cat.png", f, "image/png")
if err != nil {
	log.Fatal(err)
}

// Read the file back
data, _ := store.ReadObject("images/cat.png")
fmt.Println("Bytes read:", len(data))
```

## Using Other Providers with the `aws` Driver

The `aws` driver works with any S3-compatible storage by configuring the endpoint and style settings.

```go
store, err := objex.New(aws.Config{
	Region:       "us-east-1",
	Bucket:       "mybucket",
	AccessKey:    "minioadmin",
	SecretKey:    "minioadmin",
	Endpoint:     "localhost:9000",
	UseSSL:       false,
	UsePathStyle: true,
})
```

| Option         | Description                              |
| :------------- | :--------------------------------------- |
| `Endpoint`     | Your S3-compatible service hostname/port |
| `UseSSL`       | Set to `true` if your endpoint is HTTPS  |
| `UsePathStyle` | Required for most self-hosted providers  |


## Swapping Drivers

```go
// Swap AWS for MinIO
store := objex.New(aws.Config{...})
store = objex.New(minio.Config{...})

// They all satisfy objex.Store:
func uploadAsset(store objex.Store, name string, file io.Reader) {
	store.CreateObject(name, file, "image/png")
}
```

## Interface Overview (`objex.Store`)

```go
type Store interface {
	Setup() error
	SetBucket(name string) (bool, error)
	SetRegion(region string) error

	CreateBucket(name string) error
	DeleteBucket(name string) error
	ListBuckets() ([]Bucket, error)

	CreateObject(name string, data io.Reader, contentType string) error
	ReadObject(name string) ([]byte, error)
	UpdateObject(name string, data io.Reader) error
	DeleteObject(name string) error

	ListObjects(bucketName string) ([]*ObjectMetaData, error)
	Exists(name string) (bool, *ObjectMetaData, error)
	Metadata(name string) (*ObjectMetaData, error)

	CopyObject(src, dest string) error
	MoveObject(src, dest string) error

	CleanUp() error
	HealthCheck() error
}
```

## Testing or In-Memory Drivers

Want to use a fake/mock Store for unit tests? You can implement a dummy driver and register it with:

```go
objex.Register("mock", func(cfg any) (objex.Store, error) {
	return &MockStore{}, nil
})
```

## Reach out if you have questions or just want to chat!

- [GitHub](https://www.github.com/brian-nunez)
- [LinkedIn](https://www.linkedin.com/in/brianjnunez)

