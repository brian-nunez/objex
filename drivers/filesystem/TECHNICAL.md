# Filesystem Driver Usage

Package: `github.com/brian-nunez/objex/drivers/filesystem`

The filesystem driver maps buckets to directories and objects to files. It is
useful for local development, tests, offline applications, and simple
disk-backed deployments.

## Install

```sh
go get github.com/brian-nunez/objex@latest
go get github.com/brian-nunez/objex/drivers/filesystem@latest
```

## Configure

```go
import (
	"context"
	"log"

	"github.com/brian-nunez/objex"
	"github.com/brian-nunez/objex/drivers/filesystem"
)

ctx := context.Background()

store, err := objex.New(filesystem.Config{
	BasePath:  "./storage",
	BaseURL:   "https://files.example.com",
	SecretKey: "replace-with-a-secret",
})
if err != nil {
	log.Fatal(err)
}
```

`objex.New` returns `objex.Store`. Use `filesystem.NewStore` for a concrete
`*filesystem.Store`:

```go
store, err := filesystem.NewStore(filesystem.Config{BasePath: "./storage"})
```

### Config Fields

| Field | Usage |
| --- | --- |
| `BasePath` | Required root directory for all files and bucket directories. |
| `BaseURL` | Public or application URL prefix used only to construct presigned URLs. |
| `SecretKey` | Optional HMAC-SHA256 key used to sign filesystem URLs. |

`Config.DriverName()` returns `"filesystem"`; applications normally do not
call it directly.

## Storage Layout and Object Names

After selecting a bucket, object names are paths relative to its directory:

```go
store.SetBucket("assets")
_, err := store.CreateObject(ctx, "images/logo.png", body, "image/png")
// Written to ./storage/assets/images/logo.png
```

Without a selected bucket, a `bucket/key` name selects a directory:

```go
_, err := store.CreateObject(ctx, "assets/images/logo.png", body, "image/png")
// Written to ./storage/assets/images/logo.png
```

A plain key with no selected bucket is written directly under `BasePath`:

```go
_, err := store.CreateObject(ctx, "health.txt", body, "text/plain")
// Written to ./storage/health.txt
```

Object names are converted with `filepath.Join`. Only use names you trust; the
driver does not provide a hardened HTTP-upload boundary or explicit symlink
isolation.

## Lifecycle Functions

### `Setup`

Creates `BasePath` and missing parent directories with mode `0755`.

```go
if err := store.Setup(ctx); err != nil {
	log.Fatal(err)
}
```

### `HealthCheck`

Verifies that `BasePath` is configured and creates it if necessary.

```go
if err := store.HealthCheck(ctx); err != nil {
	log.Fatal(err)
}
```

### `CleanUp`

Does not delete files or directories; it currently returns `nil`.

```go
defer store.CleanUp()
```

Delete temporary test storage explicitly with `os.RemoveAll` when appropriate.

### `SetRegion`

The filesystem has no region. This method is a no-op for interface
compatibility.

```go
_ = store.SetRegion("us-east-1")
```

## Bucket Functions

### `SetBucket`

Creates the bucket directory if necessary and selects it.

```go
found, err := store.SetBucket("assets")
if err != nil {
	log.Fatal(err)
}
fmt.Println(found) // true when the directory was available or created
```

Unlike remote drivers, this does not distinguish an existing bucket from one
created by the call.

### `CreateBucket`

```go
if err := store.CreateBucket(ctx, "assets"); err != nil {
	log.Fatal(err)
}
```

Creates the directory and missing parents with mode `0755`.

### `DeleteBucket`

```go
if err := store.DeleteBucket(ctx, "assets"); err != nil {
	log.Fatal(err)
}
```

This recursively deletes the bucket and every object in it. It does not enforce
the empty-bucket behavior used by S3.

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

Every direct child directory of `BasePath` is treated as a bucket. The reported
creation date is the directory modification time formatted as RFC3339.

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
fmt.Println("MD5:", etag)
```

Missing directories are created automatically. The returned ETag is the
lowercase hexadecimal MD5 of the written bytes. `contentType` is accepted for
interface compatibility but is not persisted.

### `ReadObject`

```go
reader, err := store.ReadObject(ctx, "documents/report.txt")
if err != nil {
	log.Fatal(err)
}
defer reader.Close()

data, err := io.ReadAll(reader)
```

Always close the returned file.

### `ReadObjectRange`

Reads at most `length` bytes beginning at zero-based `offset`.

```go
reader, err := store.ReadObjectRange(ctx, "video.mp4", 1_048_576, 262_144)
if err != nil {
	log.Fatal(err)
}
defer reader.Close()

chunk, err := io.ReadAll(reader)
```

Use a non-negative offset and length. A range beyond the end of the file
returns the bytes that remain, possibly none.

### `UpdateObject`

Overwrites the file, creating it if it does not already exist.

```go
etag, err := store.UpdateObject(
	ctx,
	"documents/report.txt",
	strings.NewReader("revised report"),
)
```

This differs from AWS and MinIO, whose update methods require the object to
exist.

### `DeleteObject`

```go
if err := store.DeleteObject(ctx, "documents/report.txt"); err != nil {
	log.Fatal(err)
}
```

Deleting a missing file returns the operating-system error. Empty parent
directories are not removed.

### `ListObjects`

Lists files recursively.

```go
objects, err := store.ListObjects(ctx, "") // selected bucket
if err != nil {
	log.Fatal(err)
}
for _, object := range objects {
	fmt.Println(object.Key, object.Size, object.LastModified)
}
```

Without a selected bucket, pass the bucket directory:

```go
objects, err := store.ListObjects(ctx, "assets")
```

Listed content types are `application/octet-stream`. ETags are left empty to
avoid reading and hashing every file.

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

A missing file returns `false, nil, nil`. For an existing file, the driver reads
the complete file to calculate its MD5 ETag, which can be expensive for large
objects.

### `Metadata`

```go
metadata, err := store.Metadata(ctx, "images/logo.png")
if errors.Is(err, objex.ErrObjectNotFound) {
	// The file does not exist.
}
```

Metadata also hashes the complete file. Content type is reported as
`application/octet-stream`.

### `CopyObject`

```go
err := store.CopyObject(ctx, "images/logo.png", "archive/logo.png")
```

Missing destination directories are created. With no selected bucket, full
paths can copy between bucket directories:

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

Move copies the bytes, then deletes the source. It is not an atomic filesystem
rename. If deletion fails, both files may remain.

## Presigned URL Functions

Filesystem presigned URLs are tokens for an application-managed file server.
The driver only constructs URLs; it does not start an HTTP server or validate
requests.

For this config:

```go
store, err := filesystem.NewStore(filesystem.Config{
	BasePath:  "./storage",
	BaseURL:   "https://files.example.com",
	SecretKey: os.Getenv("FILESYSTEM_URL_SECRET"),
})
store.SetBucket("assets")
```

### `PresignGet`

```go
downloadURL, err := store.PresignGet(ctx, "images/logo.png", 15*time.Minute)
```

### `PresignPut`

```go
uploadURL, err := store.PresignPut(ctx, "uploads/new.png", 15*time.Minute)
```

The generated query contains `method` and Unix `expires` values. When
`SecretKey` is set, it also contains a hexadecimal HMAC-SHA256 `signature` over:

```text
METHOD:URL_PATH:EXPIRES
```

The HTTP service behind `BaseURL` must enforce the method and expiration and
verify the signature with the same secret. Without `SecretKey`, the URL is not
cryptographically signed. Treat generated URLs as temporary credentials.

## Imports Used by the Examples

```go
import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/brian-nunez/objex"
	"github.com/brian-nunez/objex/drivers/filesystem"
)
```
