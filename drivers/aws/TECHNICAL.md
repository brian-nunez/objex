# AWS Driver Usage

Package: `github.com/brian-nunez/objex/drivers/aws`

This driver uses AWS SDK for Go v2. It supports AWS S3 and S3-compatible
services that implement the operations used by the driver.

## Install

```sh
go get github.com/brian-nunez/objex@latest
go get github.com/brian-nunez/objex/drivers/aws@latest
```

## Configure

```go
import (
	"context"
	"log"
	"os"

	"github.com/brian-nunez/objex"
	objexaws "github.com/brian-nunez/objex/drivers/aws"
)

ctx := context.Background()

store, err := objex.New(objexaws.Config{
	Region:    "us-east-1",
	Bucket:    "my-app-assets",
	AccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
	SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
	Token:     os.Getenv("AWS_SESSION_TOKEN"),
})
if err != nil {
	log.Fatal(err)
}
```

`objex.New` returns `objex.Store`. Use `objexaws.NewStore` instead when a
`*objexaws.Store` is preferred:

```go
store, err := objexaws.NewStore(objexaws.Config{ /* fields */ })
```

### Config Fields

| Field | Usage |
| --- | --- |
| `Region` | Signing region. Defaults to `us-east-1`. |
| `Bucket` | Optional selected bucket. Object methods then accept keys only. |
| `Endpoint` | Optional S3-compatible host and port, without `http://` or `https://`. |
| `AccessKey` | Static access key used by this driver. |
| `SecretKey` | Static secret key used by this driver. |
| `Token` | Optional session token for temporary credentials. |
| `UseSSL` | Selects HTTPS for a custom endpoint. |
| `UsePathStyle` | Enables path-style S3 URLs, commonly required by local providers. |

S3-compatible example:

```go
store, err := objexaws.NewStore(objexaws.Config{
	Region:       "us-east-1",
	Endpoint:     "localhost:9000",
	AccessKey:    "minioadmin",
	SecretKey:    "minioadmin",
	UseSSL:       false,
	UsePathStyle: true,
})
```

`Config.DriverName()` and `Store.DriverName()` return `"aws"`. Applications
normally do not need to call them directly.

## Object Naming

With a selected bucket, names are object keys:

```go
store, _ := objexaws.NewStore(objexaws.Config{Bucket: "assets", /* ... */})
_, err := store.CreateObject(ctx, "images/logo.png", body, "image/png")
```

Without a selected bucket, include the bucket in every name:

```go
store, _ := objexaws.NewStore(objexaws.Config{ /* no Bucket */ })
_, err := store.CreateObject(ctx, "assets/images/logo.png", body, "image/png")
```

Use one style consistently. When a bucket is selected, `assets/logo.png` means
the key `assets/logo.png` inside that selected bucket.

## Lifecycle Functions

### `Setup`

AWS requires no setup; the method currently returns `nil`. Calling it keeps
driver-independent initialization code uniform.

```go
if err := store.Setup(ctx); err != nil {
	log.Fatal(err)
}
```

### `HealthCheck`

Calls S3 `ListBuckets`. The credentials therefore need permission to list
buckets, even if the application otherwise accesses only one bucket.

```go
if err := store.HealthCheck(ctx); err != nil {
	log.Printf("S3 is unavailable: %v", err)
}
```

Failures are returned as `objex.ErrClientInit`.

### `CleanUp`

The AWS client has no cleanup requirement; this currently returns `nil`.

```go
defer func() {
	if err := store.CleanUp(); err != nil {
		log.Printf("cleanup: %v", err)
	}
}()
```

### `SetRegion`

```go
err := store.SetRegion("us-west-2")
```

This only updates the store's region field. It does not rebuild the AWS client
or change its signing region. Configure `Region` before `NewStore` for actual
requests.

## Bucket Functions

### `SetBucket`

Selects a bucket for subsequent object operations and verifies it with
`HeadBucket`.

```go
found, err := store.SetBucket("assets")
if err != nil {
	log.Fatal(err)
}
fmt.Println("selected:", found)
```

The method returns `objex.ErrBucketNotFound` for any `HeadBucket` failure.

### `CreateBucket`

```go
if err := store.CreateBucket(ctx, "assets"); err != nil {
	log.Fatal(err)
}
```

An empty name returns `objex.ErrInvalidBucketName`. Any S3 create failure is
currently normalized to `objex.ErrBucketAlreadyExists`.

### `DeleteBucket`

```go
if err := store.DeleteBucket(ctx, "assets"); err != nil {
	log.Fatal(err)
}
```

S3 requires the bucket to be empty. The underlying SDK error is returned.

### `ListBuckets`

```go
buckets, err := store.ListBuckets(ctx)
if err != nil {
	log.Fatal(err)
}
for _, bucket := range buckets {
	fmt.Println(bucket.Name, bucket.CreationDate)
}
```

Creation dates use RFC3339 formatting.

## Object Functions

The snippets below assume `store` has selected the `assets` bucket.

### `CreateObject`

```go
etag, err := store.CreateObject(
	ctx,
	"documents/report.txt",
	strings.NewReader("quarterly report"),
	"text/plain",
)
if err != nil {
	log.Fatal(err)
}
fmt.Println("ETag:", etag)
```

The upload is streamed through the AWS multipart-capable uploader. The returned
ETag is the S3 response value and is not always an MD5 hash.

### `ReadObject`

```go
reader, err := store.ReadObject(ctx, "documents/report.txt")
if err != nil {
	log.Fatal(err)
}
defer reader.Close()

data, err := io.ReadAll(reader)
```

Always close the returned reader.

### `ReadObjectRange`

Reads `length` bytes beginning at the zero-based `offset`.

```go
reader, err := store.ReadObjectRange(ctx, "video.mp4", 1_048_576, 262_144)
if err != nil {
	log.Fatal(err)
}
defer reader.Close()

chunk, err := io.ReadAll(reader)
```

Callers should pass a non-negative offset and a positive length.

### `UpdateObject`

Updates an existing object while preserving its current content type.

```go
etag, err := store.UpdateObject(
	ctx,
	"documents/report.txt",
	strings.NewReader("revised report"),
)
if errors.Is(err, objex.ErrObjectNotFound) {
	// CreateObject is required for a new key.
}
```

### `DeleteObject`

```go
if err := store.DeleteObject(ctx, "documents/report.txt"); err != nil {
	log.Fatal(err)
}
```

S3 delete semantics are idempotent for a missing key unless bucket versioning
or service-specific behavior changes the request result.

### `ListObjects`

```go
objects, err := store.ListObjects(ctx, "") // uses the selected bucket
if err != nil {
	log.Fatal(err)
}
for _, object := range objects {
	fmt.Println(object.Key, object.Size, object.ETag)
}
```

If no bucket is selected, pass one as the second argument:

```go
objects, err := store.ListObjects(ctx, "assets")
```

That call also makes `assets` the store's selected bucket. Listing uses one
`ListObjectsV2` request and does not currently paginate. Listed content types
are reported as `application/octet-stream`; call `Metadata` for the stored
content type.

### `Exists`

```go
exists, metadata, err := store.Exists(ctx, "images/logo.png")
if err != nil {
	log.Fatal(err)
}
if exists {
	fmt.Println(metadata.Size, metadata.ContentType, metadata.ETag)
}
```

A recognized S3 not-found response returns `false, nil, nil`.

### `Metadata`

```go
metadata, err := store.Metadata(ctx, "images/logo.png")
if err != nil {
	log.Fatal(err)
}
if metadata == nil {
	// The current AWS implementation can return nil for a missing object.
}
```

### `CopyObject`

```go
err := store.CopyObject(ctx, "images/logo.png", "archive/logo.png")
```

With a selected bucket, both names are keys in that bucket. To copy between
buckets, use a store with no selected bucket and full `bucket/key` names:

```go
err := store.CopyObject(
	ctx,
	"source-bucket/images/logo.png",
	"destination-bucket/images/logo.png",
)
```

### `MoveObject`

```go
err := store.MoveObject(ctx, "incoming/report.pdf", "reports/report.pdf")
```

Move performs a copy followed by a delete. If deletion fails, both objects may
remain.

## Presigned URL Functions

### `PresignGet`

```go
downloadURL, err := store.PresignGet(ctx, "images/logo.png", 15*time.Minute)
```

The URL permits an HTTP GET until it expires.

### `PresignPut`

```go
uploadURL, err := store.PresignPut(ctx, "uploads/new.png", 15*time.Minute)
```

The client receiving this URL must make an HTTP PUT. Treat presigned URLs as
temporary credentials and avoid logging them.

## Imports Used by the Examples

```go
import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/brian-nunez/objex"
	objexaws "github.com/brian-nunez/objex/drivers/aws"
)
```
