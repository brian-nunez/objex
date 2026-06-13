package aws

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/brian-nunez/objex"
)

type mockS3 struct {
	listBuckets    func(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error)
	headBucket     func(ctx context.Context, params *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error)
	createBucket   func(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error)
	deleteBucket   func(ctx context.Context, params *s3.DeleteBucketInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketOutput, error)
	getObject      func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	deleteObject   func(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	listObjectsV2  func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	headObject     func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	copyObject     func(ctx context.Context, params *s3.CopyObjectInput, optFns ...func(*s3.Options)) (*s3.CopyObjectOutput, error)
	presignGet     func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
	presignPut     func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
	upload         func(ctx context.Context, input *s3.PutObjectInput, opts ...func(*manager.Uploader)) (*manager.UploadOutput, error)
}

func (m *mockS3) ListBuckets(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	return m.listBuckets(ctx, params, optFns...)
}
func (m *mockS3) HeadBucket(ctx context.Context, params *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
	return m.headBucket(ctx, params, optFns...)
}
func (m *mockS3) CreateBucket(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error) {
	return m.createBucket(ctx, params, optFns...)
}
func (m *mockS3) DeleteBucket(ctx context.Context, params *s3.DeleteBucketInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketOutput, error) {
	return m.deleteBucket(ctx, params, optFns...)
}
func (m *mockS3) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	return m.getObject(ctx, params, optFns...)
}
func (m *mockS3) DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	return m.deleteObject(ctx, params, optFns...)
}
func (m *mockS3) ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	return m.listObjectsV2(ctx, params, optFns...)
}
func (m *mockS3) HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	return m.headObject(ctx, params, optFns...)
}
func (m *mockS3) CopyObject(ctx context.Context, params *s3.CopyObjectInput, optFns ...func(*s3.Options)) (*s3.CopyObjectOutput, error) {
	return m.copyObject(ctx, params, optFns...)
}
func (m *mockS3) PresignGetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	return m.presignGet(ctx, params, optFns...)
}
func (m *mockS3) PresignPutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	return m.presignPut(ctx, params, optFns...)
}
func (m *mockS3) Upload(ctx context.Context, input *s3.PutObjectInput, opts ...func(*manager.Uploader)) (*manager.UploadOutput, error) {
	return m.upload(ctx, input, opts...)
}

type badConfig struct{}

func (b badConfig) DriverName() string { return "aws" }

func TestAWSMocked(t *testing.T) {
	mock := &mockS3{}
	store := &Store{
		client:        mock,
		presignClient: mock,
		uploader:      mock,
		bucket:        "test-bucket",
		region:        "us-east-1",
	}
	ctx := context.Background()

	t.Run("HealthCheck", func(t *testing.T) {
		mock.listBuckets = func(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
			return &s3.ListBucketsOutput{}, nil
		}
		if err := store.HealthCheck(ctx); err != nil {
			t.Errorf("HealthCheck failed: %v", err)
		}

		mock.listBuckets = func(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
			return nil, errors.New("error")
		}
		if err := store.HealthCheck(ctx); err != objex.ErrClientInit {
			t.Errorf("Expected ErrClientInit, got %v", err)
		}
	})

	t.Run("SetBucket", func(t *testing.T) {
		mock.headBucket = func(ctx context.Context, params *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
			return &s3.HeadBucketOutput{}, nil
		}
		found, err := store.SetBucket("new-bucket")
		if err != nil || !found || store.bucket != "new-bucket" {
			t.Errorf("SetBucket failed: %v, found=%v", err, found)
		}

		mock.headBucket = func(ctx context.Context, params *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
			return nil, errors.New("not found")
		}
		found, err = store.SetBucket("bad-bucket")
		if err != objex.ErrBucketNotFound || found {
			t.Errorf("Expected ErrBucketNotFound, got %v, found=%v", err, found)
		}
	})

	t.Run("SetRegion", func(t *testing.T) {
		if err := store.SetRegion("us-west-2"); err != nil || store.region != "us-west-2" {
			t.Errorf("SetRegion failed: %v", err)
		}
	})

	t.Run("CreateBucket", func(t *testing.T) {
		mock.createBucket = func(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error) {
			return &s3.CreateBucketOutput{}, nil
		}
		if err := store.CreateBucket(ctx, "new-bucket"); err != nil {
			t.Errorf("CreateBucket failed: %v", err)
		}

		if err := store.CreateBucket(ctx, ""); err != objex.ErrInvalidBucketName {
			t.Errorf("Expected ErrInvalidBucketName, got %v", err)
		}

		mock.createBucket = func(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error) {
			return nil, errors.New("already exists")
		}
		if err := store.CreateBucket(ctx, "exists"); err != objex.ErrBucketAlreadyExists {
			t.Errorf("Expected ErrBucketAlreadyExists, got %v", err)
		}
	})

	t.Run("DeleteBucket", func(t *testing.T) {
		mock.deleteBucket = func(ctx context.Context, params *s3.DeleteBucketInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketOutput, error) {
			return &s3.DeleteBucketOutput{}, nil
		}
		if err := store.DeleteBucket(ctx, "bucket"); err != nil {
			t.Errorf("DeleteBucket failed: %v", err)
		}
	})

	t.Run("ListBuckets", func(t *testing.T) {
		now := time.Now()
		mock.listBuckets = func(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
			return &s3.ListBucketsOutput{
				Buckets: []types.Bucket{
					{Name: aws.String("b1"), CreationDate: &now},
				},
			}, nil
		}
		buckets, err := store.ListBuckets(ctx)
		if err != nil || len(buckets) != 1 || buckets[0].Name != "b1" {
			t.Errorf("ListBuckets failed: %v, len=%d", err, len(buckets))
		}

		mock.listBuckets = func(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
			return nil, errors.New("error")
		}
		_, err = store.ListBuckets(ctx)
		if err == nil {
			t.Error("Expected error in ListBuckets")
		}
	})

	t.Run("CreateObject", func(t *testing.T) {
		mock.upload = func(ctx context.Context, input *s3.PutObjectInput, opts ...func(*manager.Uploader)) (*manager.UploadOutput, error) {
			return &manager.UploadOutput{ETag: aws.String("etag")}, nil
		}
		etag, err := store.CreateObject(ctx, "obj", strings.NewReader("data"), "text/plain")
		if err != nil || etag != "etag" {
			t.Errorf("CreateObject failed: %v, etag=%s", err, etag)
		}

		_, err = store.CreateObject(ctx, "", nil, "")
		if err != objex.ErrInvalidObjectName {
			t.Errorf("Expected ErrInvalidObjectName, got %v", err)
		}

		mock.upload = func(ctx context.Context, input *s3.PutObjectInput, opts ...func(*manager.Uploader)) (*manager.UploadOutput, error) {
			return nil, errors.New("error")
		}
		_, err = store.CreateObject(ctx, "obj", strings.NewReader(""), "")
		if err == nil {
			t.Error("Expected error in CreateObject")
		}
	})

	t.Run("ReadObject", func(t *testing.T) {
		mock.getObject = func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			return &s3.GetObjectOutput{Body: io.NopCloser(strings.NewReader("data"))}, nil
		}
		rc, err := store.ReadObject(ctx, "obj")
		if err != nil {
			t.Errorf("ReadObject failed: %v", err)
		} else {
			rc.Close()
		}

		_, err = store.ReadObject(ctx, "")
		if err != objex.ErrInvalidObjectName {
			t.Errorf("Expected ErrInvalidObjectName, got %v", err)
		}

		mock.getObject = func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			return nil, errors.New("error")
		}
		_, err = store.ReadObject(ctx, "obj")
		if err == nil {
			t.Error("Expected error in ReadObject")
		}
	})

	t.Run("ReadObjectRange", func(t *testing.T) {
		mock.getObject = func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			return &s3.GetObjectOutput{Body: io.NopCloser(strings.NewReader("da"))}, nil
		}
		rc, err := store.ReadObjectRange(ctx, "obj", 0, 2)
		if err != nil {
			t.Errorf("ReadObjectRange failed: %v", err)
		} else {
			rc.Close()
		}

		_, err = store.ReadObjectRange(ctx, "", 0, 2)
		if err != objex.ErrInvalidObjectName {
			t.Errorf("Expected ErrInvalidObjectName, got %v", err)
		}

		mock.getObject = func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			return nil, errors.New("error")
		}
		_, err = store.ReadObjectRange(ctx, "obj", 0, 2)
		if err == nil {
			t.Error("Expected error in ReadObjectRange")
		}
	})

	t.Run("Exists", func(t *testing.T) {
		now := time.Now()
		mock.headObject = func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
			return &s3.HeadObjectOutput{
				ContentLength: aws.Int64(4),
				ContentType:   aws.String("text/plain"),
				LastModified:  &now,
				ETag:          aws.String("etag"),
			}, nil
		}
		exists, meta, err := store.Exists(ctx, "obj")
		if err != nil || !exists || meta.Size != 4 {
			t.Errorf("Exists failed: %v, exists=%v", err, exists)
		}

		_, _, err = store.Exists(ctx, "")
		if err != objex.ErrInvalidObjectName {
			t.Errorf("Expected ErrInvalidObjectName, got %v", err)
		}

		mock.headObject = func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
			return nil, &types.NotFound{}
		}
		exists, _, err = store.Exists(ctx, "none")
		if err != nil || exists {
			t.Errorf("Expected not exists, got %v, %v", exists, err)
		}

		mock.headObject = func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
			return nil, errors.New("error")
		}
		_, _, err = store.Exists(ctx, "obj")
		if err == nil {
			t.Error("Expected error in Exists")
		}
	})

	t.Run("Metadata", func(t *testing.T) {
		mock.headObject = func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
			return &s3.HeadObjectOutput{
				ContentLength: aws.Int64(4),
				ContentType:   aws.String("text/plain"),
				LastModified:  aws.Time(time.Now()),
				ETag:          aws.String("etag"),
			}, nil
		}
		meta, err := store.Metadata(ctx, "obj")
		if err != nil || meta == nil {
			t.Errorf("Metadata failed: %v", err)
		}

		mock.headObject = func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
			return nil, &types.NotFound{}
		}
		_, err = store.Metadata(ctx, "none")
		if err != nil {
			t.Errorf("Expected nil error for not found in Metadata, got %v", err)
		}
	})

	t.Run("UpdateObject", func(t *testing.T) {
		mock.headObject = func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
			return &s3.HeadObjectOutput{
				ContentLength: aws.Int64(4),
				ContentType:   aws.String("text/plain"),
				LastModified:  aws.Time(time.Now()),
				ETag:          aws.String("etag"),
			}, nil
		}
		mock.upload = func(ctx context.Context, input *s3.PutObjectInput, opts ...func(*manager.Uploader)) (*manager.UploadOutput, error) {
			return &manager.UploadOutput{ETag: aws.String("new-etag")}, nil
		}
		etag, err := store.UpdateObject(ctx, "obj", strings.NewReader("new"))
		if err != nil || etag != "new-etag" {
			t.Errorf("UpdateObject failed: %v, etag=%s", err, etag)
		}

		mock.headObject = func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
			return nil, &types.NotFound{}
		}
		_, err = store.UpdateObject(ctx, "none", nil)
		if err != objex.ErrObjectNotFound {
			t.Errorf("Expected ErrObjectNotFound, got %v", err)
		}
	})

	t.Run("DeleteObject", func(t *testing.T) {
		mock.deleteObject = func(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
			return &s3.DeleteObjectOutput{}, nil
		}
		if err := store.DeleteObject(ctx, "obj"); err != nil {
			t.Errorf("DeleteObject failed: %v", err)
		}

		if err := store.DeleteObject(ctx, ""); err != objex.ErrInvalidObjectName {
			t.Errorf("Expected ErrInvalidObjectName, got %v", err)
		}
	})

	t.Run("ListObjects", func(t *testing.T) {
		mock.listObjectsV2 = func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{
				Contents: []types.Object{
					{Key: aws.String("k1"), Size: aws.Int64(10), LastModified: aws.Time(time.Now()), ETag: aws.String("e1")},
				},
			}, nil
		}
		objs, err := store.ListObjects(ctx, "")
		if err != nil || len(objs) != 1 {
			t.Errorf("ListObjects failed: %v, len=%d", err, len(objs))
		}

		mock.listObjectsV2 = func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return nil, errors.New("error")
		}
		_, err = store.ListObjects(ctx, "")
		if err == nil {
			t.Error("Expected error in ListObjects")
		}
	})

	t.Run("CopyObject", func(t *testing.T) {
		mock.copyObject = func(ctx context.Context, params *s3.CopyObjectInput, optFns ...func(*s3.Options)) (*s3.CopyObjectOutput, error) {
			return &s3.CopyObjectOutput{}, nil
		}
		if err := store.CopyObject(ctx, "src", "dest"); err != nil {
			t.Errorf("CopyObject failed: %v", err)
		}

		if err := store.CopyObject(ctx, "", "dest"); err != objex.ErrInvalidObjectName {
			t.Errorf("Expected ErrInvalidObjectName, got %v", err)
		}
		if err := store.CopyObject(ctx, "src", ""); err != objex.ErrInvalidObjectName {
			t.Errorf("Expected ErrInvalidObjectName, got %v", err)
		}
	})

	t.Run("MoveObject", func(t *testing.T) {
		mock.copyObject = func(ctx context.Context, params *s3.CopyObjectInput, optFns ...func(*s3.Options)) (*s3.CopyObjectOutput, error) {
			return &s3.CopyObjectOutput{}, nil
		}
		mock.deleteObject = func(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
			return &s3.DeleteObjectOutput{}, nil
		}
		if err := store.MoveObject(ctx, "src", "dest"); err != nil {
			t.Errorf("MoveObject failed: %v", err)
		}

		mock.copyObject = func(ctx context.Context, params *s3.CopyObjectInput, optFns ...func(*s3.Options)) (*s3.CopyObjectOutput, error) {
			return nil, errors.New("error")
		}
		if err := store.MoveObject(ctx, "src", "dest"); err == nil {
			t.Error("Expected error in MoveObject")
		}
	})

	t.Run("Presign", func(t *testing.T) {
		mock.presignGet = func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
			return &v4.PresignedHTTPRequest{URL: "http://get"}, nil
		}
		url, err := store.PresignGet(ctx, "obj", time.Hour)
		if err != nil || url != "http://get" {
			t.Errorf("PresignGet failed: %v, url=%s", err, url)
		}

		_, err = store.PresignGet(ctx, "", time.Hour)
		if err != objex.ErrInvalidObjectName {
			t.Errorf("Expected ErrInvalidObjectName, got %v", err)
		}

		mock.presignGet = func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
			return nil, errors.New("error")
		}
		_, err = store.PresignGet(ctx, "obj", time.Hour)
		if err == nil {
			t.Error("Expected error in PresignGet")
		}

		mock.presignPut = func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
			return &v4.PresignedHTTPRequest{URL: "http://put"}, nil
		}
		url, err = store.PresignPut(ctx, "obj", time.Hour)
		if err != nil || url != "http://put" {
			t.Errorf("PresignPut failed: %v, url=%s", err, url)
		}

		_, err = store.PresignPut(ctx, "", time.Hour)
		if err != objex.ErrInvalidObjectName {
			t.Errorf("Expected ErrInvalidObjectName, got %v", err)
		}

		mock.presignPut = func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
			return nil, errors.New("error")
		}
		_, err = store.PresignPut(ctx, "obj", time.Hour)
		if err == nil {
			t.Error("Expected error in PresignPut")
		}
	})

	t.Run("NewStore Error", func(t *testing.T) {
		// Test invalid config type for init
		_, err := objex.New(badConfig{})
		if err != objex.ErrClientInit {
			t.Errorf("Expected ErrClientInit for invalid config type, got %v", err)
		}

		// Test NewStore with Endpoint and SSL and PathStyle
		s1, _ := NewStore(Config{Endpoint: "localhost:9000", UseSSL: true, UsePathStyle: true})
		if s1 == nil {
			t.Fatal("Expected store with endpoint")
		}
		// Trigger endpoint resolver
		_, _ = s1.client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})

		// Test NewStore with default region
		s2, _ := NewStore(Config{})
		if s2.region != "us-east-1" {
			t.Errorf("Expected default region us-east-1, got %s", s2.region)
		}

		s, _ := objex.New(Config{AccessKey: "a", SecretKey: "s"})
		if s != nil && s.(*Store).DriverName() != "aws" {
			t.Error("Wrong driver name")
		}

		// Test loadDefaultConfig error
		oldLoad := loadDefaultConfig
		loadDefaultConfig = func(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
			return aws.Config{}, errors.New("error")
		}
		defer func() { loadDefaultConfig = oldLoad }()
		_, err = NewStore(Config{})
		if err != objex.ErrClientInit {
			t.Errorf("Expected ErrClientInit, got %v", err)
		}
		_, err = NewStore(Config{Endpoint: "e"})
		if err != objex.ErrClientInit {
			t.Errorf("Expected ErrClientInit for endpoint, got %v", err)
		}
	})

	t.Run("ListObjects empty store bucket", func(t *testing.T) {
		sEmpty := &Store{client: mock}
		mock.listObjectsV2 = func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			if *params.Bucket != "provided-bucket" {
				t.Errorf("Expected provided-bucket, got %s", *params.Bucket)
			}
			return &s3.ListObjectsV2Output{}, nil
		}
		sEmpty.ListObjects(ctx, "provided-bucket")
		if sEmpty.bucket != "provided-bucket" {
			t.Errorf("Expected store bucket to be updated to provided-bucket, got %s", sEmpty.bucket)
		}
	})
}
