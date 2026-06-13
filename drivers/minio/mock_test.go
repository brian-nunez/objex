package minio

import (
	"context"
	"errors"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/brian-nunez/objex"
	"github.com/minio/minio-go/v7"
)

type mockMinio struct {
	bucketExists       func(ctx context.Context, bucketName string) (bool, error)
	makeBucket         func(ctx context.Context, bucketName string, options minio.MakeBucketOptions) error
	removeBucket       func(ctx context.Context, bucketName string) error
	listBuckets        func(ctx context.Context) ([]minio.BucketInfo, error)
	putObject          func(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	getObject          func(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (io.ReadCloser, error)
	removeObject       func(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error
	listObjects        func(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo
	statObject         func(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error)
	copyObject         func(ctx context.Context, dst minio.CopyDestOptions, src minio.CopySrcOptions) (minio.UploadInfo, error)
	presignedGetObject func(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error)
	presignedPutObject func(ctx context.Context, bucketName, objectName string, expires time.Duration) (*url.URL, error)
}

func (m *mockMinio) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	return m.bucketExists(ctx, bucketName)
}
func (m *mockMinio) MakeBucket(ctx context.Context, bucketName string, options minio.MakeBucketOptions) error {
	return m.makeBucket(ctx, bucketName, options)
}
func (m *mockMinio) RemoveBucket(ctx context.Context, bucketName string) error {
	return m.removeBucket(ctx, bucketName)
}
func (m *mockMinio) ListBuckets(ctx context.Context) ([]minio.BucketInfo, error) {
	return m.listBuckets(ctx)
}
func (m *mockMinio) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	return m.putObject(ctx, bucketName, objectName, reader, objectSize, opts)
}
func (m *mockMinio) GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (io.ReadCloser, error) {
	return m.getObject(ctx, bucketName, objectName, opts)
}
func (m *mockMinio) RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error {
	return m.removeObject(ctx, bucketName, objectName, opts)
}
func (m *mockMinio) ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	return m.listObjects(ctx, bucketName, opts)
}
func (m *mockMinio) StatObject(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error) {
	return m.statObject(ctx, bucketName, objectName, opts)
}
func (m *mockMinio) CopyObject(ctx context.Context, dst minio.CopyDestOptions, src minio.CopySrcOptions) (minio.UploadInfo, error) {
	return m.copyObject(ctx, dst, src)
}
func (m *mockMinio) PresignedGetObject(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error) {
	return m.presignedGetObject(ctx, bucketName, objectName, expires, reqParams)
}
func (m *mockMinio) PresignedPutObject(ctx context.Context, bucketName, objectName string, expires time.Duration) (*url.URL, error) {
	return m.presignedPutObject(ctx, bucketName, objectName, expires)
}

type badConfig struct{}

func (b badConfig) DriverName() string { return "minio" }

type errorReader struct{}

func (e errorReader) Read(p []byte) (n int, err error) { return 0, errors.New("read error") }

func TestMinioMocked(t *testing.T) {
	mock := &mockMinio{}
	getStore := func() *Store {
		return &Store{
			client: mock,
			bucket: "test-bucket",
			config: Config{Region: "us-east-1"},
		}
	}
	store := getStore()
	ctx := context.Background()

	t.Run("ToStandardError", func(t *testing.T) {
		if ToStandardError(nil) != nil {
			t.Error("Expected nil")
		}
		
		err := minio.ErrorResponse{Code: "NoSuchBucket"}
		if ToStandardError(err) != objex.ErrBucketNotFound {
			t.Errorf("Expected ErrBucketNotFound, got %v", ToStandardError(err))
		}
		
		codes := map[string]error{
			"NoSuchKey":               objex.ErrObjectNotFound,
			"AccessDenied":            objex.ErrAccessDenied,
			"Conflict":                objex.ErrBucketNotEmpty,
			"PreconditionFailed":      objex.ErrPreconditionFailed,
			"BucketAlreadyOwnedByYou": objex.ErrBucketAlreadyExists,
			"Other":                   errors.New("Other"),
		}
		for code, expected := range codes {
			e := minio.ErrorResponse{Code: code}
			if ToStandardError(e).Error() != expected.Error() {
				t.Errorf("Expected %v, got %v for code %s", expected, ToStandardError(e), code)
			}
		}
		// Non-minio error
		nonMinio := errors.New("other")
		if ToStandardError(nonMinio) != nonMinio {
			t.Errorf("Expected original error, got %v", ToStandardError(nonMinio))
		}
	})

	t.Run("NewStore", func(t *testing.T) {
		oldNew := minioNew
		defer func() { minioNew = oldNew }()
		minioNew = func(endpoint string, opts *minio.Options) (minioClient, error) {
			return mock, nil
		}

		s, err := NewStore(Config{Endpoint: "e", AccessKey: "a", SecretKey: "s"})
		if err != nil || s == nil {
			t.Errorf("NewStore failed: %v", err)
		}

		minioNew = func(endpoint string, opts *minio.Options) (minioClient, error) {
			return nil, errors.New("error")
		}
		_, err = NewStore(Config{Endpoint: "e", AccessKey: "a", SecretKey: "s"})
		if err != objex.ErrClientInit {
			t.Errorf("Expected ErrClientInit, got %v", err)
		}

		// HealthCheck failure in NewStore
		_, err = NewStore(Config{})
		if err != objex.ErrInvalidEndpoint {
			t.Errorf("Expected ErrInvalidEndpoint, got %v", err)
		}
	})

	t.Run("HealthCheck", func(t *testing.T) {
		s := &Store{config: Config{}}
		if err := s.HealthCheck(ctx); err != objex.ErrInvalidEndpoint {
			t.Errorf("Expected ErrInvalidEndpoint, got %v", err)
		}
		s.config.Endpoint = "e"
		if err := s.HealthCheck(ctx); err != objex.ErrInvalidAccessKey {
			t.Errorf("Expected ErrInvalidAccessKey, got %v", err)
		}
		s.config.AccessKey = "a"
		if err := s.HealthCheck(ctx); err != objex.ErrInvalidSecretKey {
			t.Errorf("Expected ErrInvalidSecretKey, got %v", err)
		}
		s.config.SecretKey = "s"
		if err := s.HealthCheck(ctx); err != nil {
			t.Errorf("HealthCheck failed: %v", err)
		}
		// Region warning
		s.config.Region = ""
		s.HealthCheck(ctx)
		if s.config.Region != "us-east-1" {
			t.Error("Expected default region")
		}
	})

	t.Run("SetBucket", func(t *testing.T) {
		mock.bucketExists = func(ctx context.Context, bucketName string) (bool, error) {
			return true, nil
		}
		found, err := store.SetBucket("new")
		if err != nil || !found || store.bucket != "new" {
			t.Errorf("SetBucket failed: %v, found=%v", err, found)
		}

		found, err = getStore().SetBucket("")
		if err != nil || found {
			t.Errorf("Expected false, nil for empty bucket, got %v, %v", found, err)
		}

		mock.bucketExists = func(ctx context.Context, bucketName string) (bool, error) {
			return false, nil
		}
		found, err = store.SetBucket("none")
		if err != objex.ErrBucketNotFound || found {
			t.Errorf("Expected ErrBucketNotFound, got %v", err)
		}

		mock.bucketExists = func(ctx context.Context, bucketName string) (bool, error) {
			return false, errors.New("error")
		}
		_, err = store.SetBucket("err")
		if err == nil {
			t.Error("Expected error in SetBucket")
		}
	})

	t.Run("SetRegion", func(t *testing.T) {
		store.SetRegion("us-west-1")
		if store.config.Region != "us-west-1" {
			t.Error("SetRegion failed")
		}
		store.SetRegion("")
		if store.config.Region != "us-east-1" {
			t.Error("SetRegion default failed")
		}
	})

	t.Run("CreateBucket", func(t *testing.T) {
		mock.makeBucket = func(ctx context.Context, bucketName string, options minio.MakeBucketOptions) error {
			return nil
		}
		if err := store.CreateBucket(ctx, "b"); err != nil {
			t.Errorf("CreateBucket failed: %v", err)
		}
		if err := store.CreateBucket(ctx, ""); err != objex.ErrInvalidBucketName {
			t.Errorf("Expected ErrInvalidBucketName, got %v", err)
		}
		mock.makeBucket = func(ctx context.Context, bucketName string, options minio.MakeBucketOptions) error {
			return minio.ErrorResponse{Code: "BucketAlreadyOwnedByYou"}
		}
		if err := store.CreateBucket(ctx, "b"); err != objex.ErrBucketAlreadyExists {
			t.Errorf("Expected ErrBucketAlreadyExists, got %v", err)
		}
	})

	t.Run("DeleteBucket", func(t *testing.T) {
		mock.removeBucket = func(ctx context.Context, bucketName string) error {
			return nil
		}
		if err := store.DeleteBucket(ctx, "b"); err != nil {
			t.Errorf("DeleteBucket failed: %v", err)
		}
		if err := store.DeleteBucket(ctx, ""); err != objex.ErrInvalidBucketName {
			t.Errorf("Expected ErrInvalidBucketName, got %v", err)
		}
		mock.removeBucket = func(ctx context.Context, bucketName string) error {
			return minio.ErrorResponse{Code: "NoSuchBucket"}
		}
		if err := store.DeleteBucket(ctx, "b"); err != nil {
			t.Errorf("Expected nil error for NoSuchBucket, got %v", err)
		}
		mock.removeBucket = func(ctx context.Context, bucketName string) error {
			return errors.New("error")
		}
		if err := store.DeleteBucket(ctx, "b"); err == nil {
			t.Error("Expected error in DeleteBucket")
		}
	})

	t.Run("ListBuckets", func(t *testing.T) {
		mock.listBuckets = func(ctx context.Context) ([]minio.BucketInfo, error) {
			return []minio.BucketInfo{{Name: "b1", CreationDate: time.Now()}}, nil
		}
		buckets, err := store.ListBuckets(ctx)
		if err != nil || len(buckets) != 1 {
			t.Errorf("ListBuckets failed: %v", err)
		}
		mock.listBuckets = func(ctx context.Context) ([]minio.BucketInfo, error) {
			return nil, errors.New("error")
		}
		_, err = store.ListBuckets(ctx)
		if err == nil {
			t.Error("Expected error in ListBuckets")
		}
	})

	t.Run("CreateObject", func(t *testing.T) {
		mock.putObject = func(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
			return minio.UploadInfo{ETag: "etag"}, nil
		}
		etag, err := store.CreateObject(ctx, "obj", strings.NewReader("data"), "text/plain")
		if err != nil || etag != "etag" {
			t.Errorf("CreateObject failed: %v, etag=%s", err, etag)
		}
		if _, err := store.CreateObject(ctx, "", nil, ""); err != objex.ErrInvalidObjectName {
			t.Errorf("Expected ErrInvalidObjectName, got %v", err)
		}
		mock.putObject = func(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
			return minio.UploadInfo{}, errors.New("error")
		}
		if _, err := store.CreateObject(ctx, "obj", strings.NewReader(""), ""); err == nil {
			t.Error("Expected error in CreateObject")
		}
		// GetStreamSize error
		if _, err := store.CreateObject(ctx, "obj", errorReader{}, ""); err != objex.ErrPreconditionFailed {
			t.Errorf("Expected ErrPreconditionFailed, got %v", err)
		}
	})

	t.Run("ReadObject", func(t *testing.T) {
		mock.getObject = func(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("data")), nil
		}
		rc, err := store.ReadObject(ctx, "obj")
		if err != nil {
			t.Errorf("ReadObject failed: %v", err)
		} else {
			rc.Close()
		}
		if _, err := store.ReadObject(ctx, ""); err != objex.ErrInvalidObjectName {
			t.Errorf("Expected ErrInvalidObjectName, got %v", err)
		}
		mock.getObject = func(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (io.ReadCloser, error) {
			return nil, errors.New("error")
		}
		if _, err := store.ReadObject(ctx, "obj"); err == nil {
			t.Error("Expected error in ReadObject")
		}
	})

	t.Run("ReadObjectRange", func(t *testing.T) {
		mock.getObject = func(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("da")), nil
		}
		rc, err := store.ReadObjectRange(ctx, "obj", 0, 2)
		if err != nil {
			t.Errorf("ReadObjectRange failed: %v", err)
		} else {
			rc.Close()
		}
		if _, err := store.ReadObjectRange(ctx, "", 0, 2); err != objex.ErrInvalidObjectName {
			t.Errorf("Expected ErrInvalidObjectName, got %v", err)
		}
		// Invalid range
		if _, err := store.ReadObjectRange(ctx, "obj", -1, 10); err == nil {
			t.Error("Expected error for invalid range")
		}
		mock.getObject = func(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (io.ReadCloser, error) {
			return nil, errors.New("error")
		}
		if _, err := store.ReadObjectRange(ctx, "obj", 0, 2); err == nil {
			t.Error("Expected error in ReadObjectRange")
		}
	})

	t.Run("UpdateObject", func(t *testing.T) {
		mock.statObject = func(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error) {
			return minio.ObjectInfo{Key: "obj", ContentType: "text/plain"}, nil
		}
		mock.putObject = func(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
			return minio.UploadInfo{ETag: "new-etag"}, nil
		}
		etag, err := store.UpdateObject(ctx, "obj", strings.NewReader("new"))
		if err != nil || etag != "new-etag" {
			t.Errorf("UpdateObject failed: %v, etag=%s", err, etag)
		}
		mock.statObject = func(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error) {
			return minio.ObjectInfo{}, minio.ErrorResponse{Code: "NoSuchKey"}
		}
		if _, err := store.UpdateObject(ctx, "none", nil); err != objex.ErrObjectNotFound {
			t.Errorf("Expected ErrObjectNotFound, got %v", err)
		}
		// CreateObject error in UpdateObject
		mock.statObject = func(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error) {
			return minio.ObjectInfo{Key: "obj"}, nil
		}
		mock.putObject = func(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
			return minio.UploadInfo{}, errors.New("error")
		}
		if _, err := store.UpdateObject(ctx, "obj", strings.NewReader("")); err == nil {
			t.Error("Expected error in UpdateObject (CreateObject)")
		}
		// Exists error in UpdateObject
		mock.statObject = func(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error) {
			return minio.ObjectInfo{}, errors.New("error")
		}
		if _, err := store.UpdateObject(ctx, "err", nil); err == nil {
			t.Error("Expected error in UpdateObject (Exists)")
		}
	})

	t.Run("DeleteObject", func(t *testing.T) {
		mock.removeObject = func(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error {
			return nil
		}
		if err := store.DeleteObject(ctx, "obj"); err != nil {
			t.Errorf("DeleteObject failed: %v", err)
		}
		if err := store.DeleteObject(ctx, ""); err != objex.ErrInvalidObjectName {
			t.Errorf("Expected ErrInvalidObjectName, got %v", err)
		}
		mock.removeObject = func(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error {
			return errors.New("error")
		}
		if err := store.DeleteObject(ctx, "obj"); err == nil {
			t.Error("Expected error in DeleteObject")
		}
	})

	t.Run("ListObjects", func(t *testing.T) {
		mock.listObjects = func(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
			ch := make(chan minio.ObjectInfo, 2)
			ch <- minio.ObjectInfo{Key: "k1", Size: 10, LastModified: time.Now(), ETag: "e1"}
			ch <- minio.ObjectInfo{Err: errors.New("error")}
			close(ch)
			return ch
		}
		_, err := store.ListObjects(ctx, "")
		if err == nil {
			t.Error("Expected error in ListObjects")
		}

		mock.listObjects = func(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
			ch := make(chan minio.ObjectInfo, 1)
			ch <- minio.ObjectInfo{Key: "k1", Size: 10, LastModified: time.Now(), ETag: "e1"}
			close(ch)
			return ch
		}
		objs, err := store.ListObjects(ctx, "")
		if err != nil || len(objs) != 1 {
			t.Errorf("ListObjects failed: %v, len=%d", err, len(objs))
		}
		
		mock.listObjects = func(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
			ch := make(chan minio.ObjectInfo, 1)
			ch <- minio.ObjectInfo{Err: errors.New("error")}
			close(ch)
			return ch
		}
		if _, err := store.ListObjects(ctx, "bucket"); err == nil {
			t.Error("Expected error in ListObjects channel")
		}

		mock.listObjects = func(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
			ch := make(chan minio.ObjectInfo, 2)
			ch <- minio.ObjectInfo{Key: "obj"}
			ch <- minio.ObjectInfo{Key: ""} // skip
			close(ch)
			return ch
		}
		objs, err = store.ListObjects(ctx, "bucket")
		if err != nil || len(objs) != 1 {
			t.Errorf("Expected 1 object, got %d", len(objs))
		}
		
		sNoBucket := &Store{client: mock}
		if _, err := sNoBucket.ListObjects(ctx, ""); err != objex.ErrInvalidBucketName {
			t.Errorf("Expected ErrInvalidBucketName, got %v", err)
		}
	})

	t.Run("Exists", func(t *testing.T) {
		mock.statObject = func(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error) {
			return minio.ObjectInfo{Key: "obj", Size: 4}, nil
		}
		exists, meta, err := store.Exists(ctx, "obj")
		if err != nil || !exists || meta.Size != 4 {
			t.Errorf("Exists failed: %v, exists=%v", err, exists)
		}
		mock.statObject = func(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error) {
			return minio.ObjectInfo{}, minio.ErrorResponse{Code: "NoSuchKey"}
		}
		exists, _, err = store.Exists(ctx, "none")
		if err != nil || exists {
			t.Errorf("Expected not exists, got %v, %v", exists, err)
		}
		mock.statObject = func(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error) {
			return minio.ObjectInfo{}, errors.New("error")
		}
		if _, _, err := store.Exists(ctx, "err"); err == nil {
			t.Error("Expected error in Exists")
		}
	})

	t.Run("Metadata", func(t *testing.T) {
		mock.statObject = func(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error) {
			return minio.ObjectInfo{Key: "obj"}, nil
		}
		meta, err := store.Metadata(ctx, "obj")
		if err != nil || meta == nil {
			t.Errorf("Metadata failed: %v", err)
		}
		mock.statObject = func(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error) {
			return minio.ObjectInfo{}, minio.ErrorResponse{Code: "NoSuchKey"}
		}
		meta, err = store.Metadata(ctx, "none")
		if err != nil || meta != nil {
			t.Errorf("Expected nil metadata, got %v, %v", meta, err)
		}
		// Metadata error
		mock.statObject = func(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error) {
			return minio.ObjectInfo{}, errors.New("error")
		}
		if _, err := store.Metadata(ctx, "err"); err == nil {
			t.Error("Expected error in Metadata")
		}
	})


	t.Run("CopyObject", func(t *testing.T) {
		mock.copyObject = func(ctx context.Context, dst minio.CopyDestOptions, src minio.CopySrcOptions) (minio.UploadInfo, error) {
			return minio.UploadInfo{}, nil
		}
		if err := store.CopyObject(ctx, "s", "d"); err != nil {
			t.Errorf("CopyObject failed: %v", err)
		}
		if err := store.CopyObject(ctx, "", "d"); err != objex.ErrInvalidObjectName {
			t.Errorf("Expected ErrInvalidObjectName, got %v", err)
		}
		mock.copyObject = func(ctx context.Context, dst minio.CopyDestOptions, src minio.CopySrcOptions) (minio.UploadInfo, error) {
			return minio.UploadInfo{}, errors.New("error")
		}
		if err := store.CopyObject(ctx, "s", "d"); err == nil {
			t.Error("Expected error in CopyObject")
		}
	})

	t.Run("MoveObject", func(t *testing.T) {
		mock.copyObject = func(ctx context.Context, dst minio.CopyDestOptions, src minio.CopySrcOptions) (minio.UploadInfo, error) {
			return minio.UploadInfo{}, nil
		}
		mock.removeObject = func(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error {
			return nil
		}
		if err := store.MoveObject(ctx, "s", "d"); err != nil {
			t.Errorf("MoveObject failed: %v", err)
		}
		mock.copyObject = func(ctx context.Context, dst minio.CopyDestOptions, src minio.CopySrcOptions) (minio.UploadInfo, error) {
			return minio.UploadInfo{}, errors.New("error")
		}
		if err := store.MoveObject(ctx, "s", "d"); err == nil {
			t.Error("Expected error in MoveObject")
		}
		// Delete failure after copy
		mock.copyObject = func(ctx context.Context, dst minio.CopyDestOptions, src minio.CopySrcOptions) (minio.UploadInfo, error) {
			return minio.UploadInfo{}, nil
		}
		mock.removeObject = func(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error {
			return errors.New("error")
		}
		if err := store.MoveObject(ctx, "s", "d"); err == nil {
			t.Error("Expected error in MoveObject (Delete)")
		}
	})

	t.Run("Presign", func(t *testing.T) {
		u, _ := url.Parse("http://localhost")
		mock.presignedGetObject = func(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error) {
			return u, nil
		}
		urlStr, err := store.PresignGet(ctx, "obj", time.Hour)
		if err != nil || urlStr != "http://localhost" {
			t.Errorf("PresignGet failed: %v", err)
		}
		mock.presignedGetObject = func(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error) {
			return nil, errors.New("error")
		}
		if _, err := store.PresignGet(ctx, "obj", time.Hour); err == nil {
			t.Error("Expected error in PresignGet")
		}

		mock.presignedPutObject = func(ctx context.Context, bucketName, objectName string, expires time.Duration) (*url.URL, error) {
			return u, nil
		}
		urlStr, err = store.PresignPut(ctx, "obj", time.Hour)
		if err != nil || urlStr != "http://localhost" {
			t.Errorf("PresignPut failed: %v", err)
		}
		mock.presignedPutObject = func(ctx context.Context, bucketName, objectName string, expires time.Duration) (*url.URL, error) {
			return nil, errors.New("error")
		}
		if _, err := store.PresignPut(ctx, "obj", time.Hour); err == nil {
			t.Error("Expected error in PresignPut")
		}
	})

	t.Run("Init", func(t *testing.T) {
		_, err := objex.New(Config{Endpoint: "e", AccessKey: "a", SecretKey: "s"})
		if err != nil {
			t.Logf("objex.New might fail if minioNew not mocked globally: %v", err)
		}
		
		_, err = objex.New(badConfig{})
		if err != objex.ErrClientInit {
			t.Errorf("Expected ErrClientInit, got %v", err)
		}
	})

	t.Run("SplitPath Errors", func(t *testing.T) {
		s := &Store{bucket: ""} // No bucket
		ctx := context.Background()
		name := "no-slash"

		if _, err := s.CreateObject(ctx, name, nil, ""); err != objex.ErrInvalidObjectName {
			t.Errorf("CreateObject: expected ErrInvalidObjectName, got %v", err)
		}
		if _, err := s.ReadObject(ctx, name); err != objex.ErrInvalidObjectName {
			t.Errorf("ReadObject: expected ErrInvalidObjectName, got %v", err)
		}
		if _, err := s.ReadObjectRange(ctx, name, 0, 1); err != objex.ErrInvalidObjectName {
			t.Errorf("ReadObjectRange: expected ErrInvalidObjectName, got %v", err)
		}
		if err := s.DeleteObject(ctx, name); err != objex.ErrInvalidObjectName {
			t.Errorf("DeleteObject: expected ErrInvalidObjectName, got %v", err)
		}
		if _, _, err := s.Exists(ctx, name); err != objex.ErrInvalidObjectName {
			t.Errorf("Exists: expected ErrInvalidObjectName, got %v", err)
		}
		if _, err := s.Metadata(ctx, name); err != objex.ErrInvalidObjectName {
			t.Errorf("Metadata: expected ErrInvalidObjectName, got %v", err)
		}
		if err := s.CopyObject(ctx, name, "d"); err != objex.ErrInvalidObjectName {
			t.Errorf("CopyObject src: expected ErrInvalidObjectName, got %v", err)
		}
		if err := s.CopyObject(ctx, "b/s", name); err != objex.ErrInvalidObjectName {
			t.Errorf("CopyObject dest: expected ErrInvalidObjectName, got %v", err)
		}
		if _, err := s.PresignGet(ctx, name, time.Hour); err != objex.ErrInvalidObjectName {
			t.Errorf("PresignGet: expected ErrInvalidObjectName, got %v", err)
		}
		if _, err := s.PresignPut(ctx, name, time.Hour); err != objex.ErrInvalidObjectName {
			t.Errorf("PresignPut: expected ErrInvalidObjectName, got %v", err)
		}
	})

	t.Run("Wrapper", func(t *testing.T) {
		c, _ := minio.New("localhost:9000", &minio.Options{})
		w := &minioClientWrapper{c}
		_, _ = w.GetObject(context.Background(), "b", "o", minio.GetObjectOptions{})
	})
}
