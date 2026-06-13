# MinIO Driver Usage

Package: `github.com/brian-nunez/objex/drivers/minio`

This driver uses the MinIO Go SDK and is intended for MinIO object servers.

## Install

```sh
go get github.com/brian-nunez/objex@latest
go get github.com/brian-nunez/objex/drivers/minio@latest
```

## Configure

```go
import (
	"context"
	"log"
	"os"

	"github.com/brian-nunez/objex"
	objexminio "github.com/brian-nunez/objex/drivers/minio"
)

ctx := context.Background()

store, err := objex.New(objexminio.Config{
	Endpoint:  "localhost:9000",
	AccessKey: os.Getenv("MINIO_ACCESS_KEY"),
	SecretKey: os.Getenv("MINIO_SECRET_KEY"),
	UseSSL:    false,
	Region:    "us-east-1",
})
if err != nil {
	log.Fatal(err)
}
```

`objex.New` returns `objex.Store`. Use `objexminio.NewStore` for a concrete
`*objexminio.Store`:

```go
store, err := objexminio.NewStore(objexminio.Config{ /* fields */ })
```

### Config Fields

| Field | Usage |
| --- | --- |
| `Endpoint` | Required host and port, without a URL scheme. |
| `AccessKey` | Required static access key. |
| `SecretKey` | Required static secret key. |
| `Token` | Optional session token. |
| `UseSSL` | Uses HTTPS when `true`. |
| `Region` | Bucket creation region. Defaults to `us-east-1`. |
| `UsePathStyle` | Present for config compatibility; currently unused by this driver. |

`Config.DriverName()` returns `"minio"`; applications normally do not call it
directly.

## Object Naming

Select a bucket, then pass object keys:

```go
found, err := store.SetBucket("assets")
etag, err := store.CreateObject(ctx, "images/logo.png", body, "image/png")
```

Or leave the bucket unset and pass `bucket/key` to each object method:

```go
etag, err := store.CreateObject(
	ctx,
	"assets/images/logo.png",
	body,
	"image/png",
)
```

## Lifecycle Functions

### `Setup`

MinIO requires no setup; the method currently returns `nil`.

```go
if err := store.Setup(ctx); err != nil {
	log.Fatal(err)
}
```

### `HealthCheck`

Validates the local configuration. It does not send a request to the MinIO
server.

```go
if err := store.HealthCheck(ctx); err != nil {
	log.Fatal(err)
}
```

It returns `objex.ErrInvalidEndpoint`, `objex.ErrInvalidAccessKey`, or
`objex.ErrInvalidSecretKey` for missing required fields. An empty region is set
to `us-east-1`.

### `CleanUp`

The MinIO client has no cleanup requirement; this currently returns `nil`.

```go
defer store.CleanUp()
```

### `SetRegion`

Sets the region used by later `CreateBucket` calls. An empty value resets it to
`us-east-1`.

```go
if err := store.SetRegion("us-west-2"); err != nil {
	log.Fatal(err)
}
```

## Bucket Functions

### `SetBucket`

Checks that a bucket exists and selects it for subsequent object operations.

```go
found, err := store.SetBucket("assets")
if errors.Is(err, objex.ErrBucketNotFound) {
	// Create the bucket before selecting it.
}
fmt.Println("selected:", found)
```

Passing an empty name clears the selected bucket and returns `false, nil`.

### `CreateBucket`

```go
if err := store.CreateBucket(ctx, "assets"); err != nil {
	log.Fatal(err)
}
```

An empty name returns `objex.ErrInvalidBucketName`.

### `DeleteBucket`

```go
if err := store.DeleteBucket(ctx, "assets"); err != nil {
	log.Fatal(err)
}
```

Deleting a missing bucket succeeds. Deleting a non-empty bucket returns a
normalized MinIO error when the server provides one.

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

## Object Functions

The snippets below assume the `assets` bucket is selected.

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

An empty content type defaults to `application/octet-stream`. For readers that
do not expose a size or implement `io.Seeker`, the driver buffers the complete
input in memory before uploading it.

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

Reads `length` bytes starting at zero-based `offset`.

```go
reader, err := store.ReadObjectRange(ctx, "video.mp4", 1_048_576, 262_144)
if err != nil {
	log.Fatal(err)
}
defer reader.Close()

chunk, err := io.ReadAll(reader)
```

Use a non-negative offset and a positive length.

### `UpdateObject`

Updates an existing object and preserves the content type returned by MinIO.

```go
etag, err := store.UpdateObject(
	ctx,
	"documents/report.txt",
	strings.NewReader("revised report"),
)
if errors.Is(err, objex.ErrObjectNotFound) {
	// Use CreateObject for a new key.
}
```

### `DeleteObject`

```go
if err := store.DeleteObject(ctx, "documents/report.txt"); err != nil {
	log.Fatal(err)
}
```

### `ListObjects`

Lists recursively.

```go
objects, err := store.ListObjects(ctx, "") // selected bucket
if err != nil {
	log.Fatal(err)
}
for _, object := range objects {
	fmt.Println(object.Key, object.Size, object.ContentType)
}
```

Without a selected bucket, pass the bucket name:

```go
objects, err := store.ListObjects(ctx, "assets")
```

If neither source provides a bucket, the method returns
`objex.ErrInvalidBucketName`.

### `Exists`

```go
exists, metadata, err := store.Exists(ctx, "images/logo.png")
if err != nil {
	log.Fatal(err)
}
if exists {
	fmt.Println(metadata.Size, metadata.ETag)
}
```

A missing object returns `false, nil, nil`.

### `Metadata`

```go
metadata, err := store.Metadata(ctx, "images/logo.png")
if err != nil {
	log.Fatal(err)
}
if metadata == nil {
	// The object does not exist.
}
```

### `CopyObject`

```go
err := store.CopyObject(ctx, "images/logo.png", "archive/logo.png")
```

With no selected bucket, full paths allow cross-bucket copies:

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

### `PresignPut`

```go
uploadURL, err := store.PresignPut(ctx, "uploads/new.png", 15*time.Minute)
```

Treat presigned URLs as temporary credentials. The URL's hostname is derived
from the configured MinIO endpoint.

## Error Normalization

`ToStandardError` is public for callers that also make direct MinIO SDK calls
and want the driver's normalization:

```go
if err := objexminio.ToStandardError(minioErr); err != nil {
	if errors.Is(err, objex.ErrAccessDenied) {
		// Handle access denial.
	}
}
```

It maps common MinIO codes to Objex sentinel errors and returns other MinIO
codes as errors containing the code.

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
	objexminio "github.com/brian-nunez/objex/drivers/minio"
)
```
