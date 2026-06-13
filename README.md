# objex

`objex` is a Go interface for object storage. Applications can use the same
`objex.Store` API with AWS S3, MinIO, or the local filesystem.

The current API supports:

- Context-aware bucket and object operations
- Streaming uploads and downloads
- Byte-range reads
- Object metadata and existence checks
- Copy and move operations
- Presigned GET and PUT URLs
- ETags returned from creates and updates

## Requirements

- Go 1.22 or newer (the MinIO driver currently declares Go 1.23)
- Credentials and a reachable service for AWS S3 or MinIO
- A writable directory for the filesystem driver

## Install

Install the core package and only the driver your application needs:

```sh
go get github.com/brian-nunez/objex@latest
go get github.com/brian-nunez/objex/drivers/aws@latest
# or
go get github.com/brian-nunez/objex/drivers/minio@latest
# or
go get github.com/brian-nunez/objex/drivers/filesystem@latest
```

Each driver is a separate Go module and registers itself when imported.

## Supported Drivers

| Driver | Package | Intended use |
| --- | --- | --- |
| AWS | `github.com/brian-nunez/objex/drivers/aws` | AWS S3 and S3-compatible services |
| MinIO | `github.com/brian-nunez/objex/drivers/minio` | MinIO servers using the MinIO Go SDK |
| Filesystem | `github.com/brian-nunez/objex/drivers/filesystem` | Local development, tests, and disk-backed storage |

Detailed consumer documentation:

- [AWS driver](drivers/aws/TECHNICAL.md)
- [MinIO driver](drivers/minio/TECHNICAL.md)
- [Filesystem driver](drivers/filesystem/TECHNICAL.md)

## Quick Start

The example below uses the filesystem driver, so it does not require an
external service.

```go
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/brian-nunez/objex"
	"github.com/brian-nunez/objex/drivers/filesystem"
)

func main() {
	ctx := context.Background()

	store, err := objex.New(filesystem.Config{BasePath: "./storage"})
	if err != nil {
		log.Fatal(err)
	}
	if err := store.Setup(ctx); err != nil {
		log.Fatal(err)
	}
	if _, err := store.SetBucket("assets"); err != nil {
		log.Fatal(err)
	}

	etag, err := store.CreateObject(
		ctx,
		"notes/hello.txt",
		strings.NewReader("hello from objex"),
		"text/plain",
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("ETag:", etag)

	reader, err := store.ReadObject(ctx, "notes/hello.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(data))
}
```

`ReadObject` and `ReadObjectRange` return an `io.ReadCloser`. The caller must
close it.

## Constructing a Store

Use `objex.New` when application code should depend on `objex.Store`:

```go
store, err := objex.New(filesystem.Config{BasePath: "./storage"})
```

Use a driver's `NewStore` when the concrete driver type is useful:

```go
store, err := filesystem.NewStore(filesystem.Config{BasePath: "./storage"})
```

Both forms create the same driver implementation. `objex.New` returns
`objex.ErrUnknownDriver` when the config's driver has not been registered.

## Object Names and Buckets

AWS and MinIO use the same object naming rule:

- After `SetBucket("assets")`, pass keys such as `images/logo.png`.
- Without a selected bucket, pass `bucket/key`, such as
  `assets/images/logo.png`.

The filesystem driver also accepts `bucket/key` when no bucket is selected.
A plain key without a selected bucket is stored directly under `BasePath`.

```go
// Selected-bucket style.
_, err := store.SetBucket("assets")
etag, err := store.CreateObject(ctx, "images/logo.png", file, "image/png")

// Full-path style. Create a separate store without calling SetBucket.
etag, err = store.CreateObject(ctx, "assets/images/logo.png", file, "image/png")
```

## Store API

```go
type Store interface {
	Setup(ctx context.Context) error
	SetBucket(bucketName string) (bool, error)
	SetRegion(region string) error

	CreateBucket(ctx context.Context, bucketName string) error
	DeleteBucket(ctx context.Context, bucketName string) error
	ListBuckets(ctx context.Context) ([]Bucket, error)

	CreateObject(ctx context.Context, objectName string, data io.Reader, contentType string) (string, error)
	ReadObject(ctx context.Context, objectName string) (io.ReadCloser, error)
	ReadObjectRange(ctx context.Context, objectName string, offset, length int64) (io.ReadCloser, error)
	UpdateObject(ctx context.Context, objectName string, data io.Reader) (string, error)
	DeleteObject(ctx context.Context, objectName string) error
	ListObjects(ctx context.Context, bucketName string) ([]*ObjectMetaData, error)
	Exists(ctx context.Context, objectName string) (bool, *ObjectMetaData, error)
	Metadata(ctx context.Context, objectName string) (*ObjectMetaData, error)
	CopyObject(ctx context.Context, source, destination string) error
	MoveObject(ctx context.Context, source, destination string) error

	PresignGet(ctx context.Context, objectName string, expiration time.Duration) (string, error)
	PresignPut(ctx context.Context, objectName string, expiration time.Duration) (string, error)

	CleanUp() error
	HealthCheck(ctx context.Context) error
}
```

`CreateObject` and `UpdateObject` return the backend's ETag. ETag format and
semantics are backend-specific and should not be treated as a universal content
hash.

## Metadata

Bucket creation dates and object modification dates are strings because each
driver preserves its backend's representation.

```go
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
```

`ListObjects` may return less detailed metadata than `Metadata`. For example,
the AWS and filesystem drivers report `application/octet-stream` while listing,
and filesystem listings omit ETags to avoid hashing every file.

## Errors

Drivers use the shared sentinel errors where they can normalize backend
behavior:

```go
if errors.Is(err, objex.ErrObjectNotFound) {
	// Handle a missing object.
}
```

Available sentinel errors include `ErrUnknownDriver`, `ErrInvalidEndpoint`,
`ErrInvalidAccessKey`, `ErrInvalidSecretKey`, `ErrClientInit`,
`ErrBucketNotFound`, `ErrInvalidBucketName`, `ErrObjectNotFound`,
`ErrAccessDenied`, `ErrBucketNotEmpty`, `ErrPreconditionFailed`,
`ErrBucketAlreadyExists`, `ErrInvalidObjectName`, and `ErrInvalidFile`.

Some AWS SDK and operating-system errors are returned directly, so callers
should not assume every failure maps to a sentinel error.

## Driver Swapping

Keep storage-dependent application code typed against `objex.Store`:

```go
func putAvatar(ctx context.Context, store objex.Store, r io.Reader) error {
	_, err := store.CreateObject(ctx, "avatars/current.png", r, "image/png")
	return err
}
```

Then select a driver at composition time:

```go
var store objex.Store

store, err = objex.New(aws.Config{ /* ... */ })
store, err = objex.New(minio.Config{ /* ... */ })
store, err = objex.New(filesystem.Config{ /* ... */ })
```

## Implementing a Custom Driver

Implement `objex.Store`, provide a config implementing `DriverName`, and
register a constructor:

```go
type Config struct{}

func (Config) DriverName() string { return "custom" }

func init() {
	objex.Register("custom", func(config any) (objex.Store, error) {
		cfg, ok := config.(Config)
		if !ok {
			return nil, objex.ErrClientInit
		}
		return NewStore(cfg)
	})
}
```

Registration mutates a package-level registry and is intended to happen during
package initialization, before concurrent calls to `objex.New`.

## License

See [LICENSE](LICENSE).
