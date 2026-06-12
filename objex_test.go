package objex

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

type mockConfig struct {
	name string
}

func (m mockConfig) DriverName() string {
	return m.name
}

type mockStore struct{}

func (m *mockStore) Setup(ctx context.Context) error                             { return nil }
func (m *mockStore) SetBucket(bucketName string) (bool, error)                   { return true, nil }
func (m *mockStore) SetRegion(region string) error                               { return nil }
func (m *mockStore) CreateBucket(ctx context.Context, name string) error          { return nil }
func (m *mockStore) DeleteBucket(ctx context.Context, name string) error          { return nil }
func (m *mockStore) ListBuckets(ctx context.Context) ([]Bucket, error)           { return nil, nil }
func (m *mockStore) CreateObject(ctx context.Context, name string, data io.Reader, contentType string) (string, error) {
	return "etag", nil
}
func (m *mockStore) ReadObject(ctx context.Context, name string) (io.ReadCloser, error) { return nil, nil }
func (m *mockStore) ReadObjectRange(ctx context.Context, name string, offset, length int64) (io.ReadCloser, error) {
	return nil, nil
}
func (m *mockStore) UpdateObject(ctx context.Context, name string, data io.Reader) (string, error) {
	return "etag", nil
}
func (m *mockStore) DeleteObject(ctx context.Context, name string) error { return nil }
func (m *mockStore) ListObjects(ctx context.Context, bucket string) ([]*ObjectMetaData, error) {
	return nil, nil
}
func (m *mockStore) Exists(ctx context.Context, name string) (bool, *ObjectMetaData, error) {
	return false, nil, nil
}
func (m *mockStore) Metadata(ctx context.Context, name string) (*ObjectMetaData, error) { return nil, nil }
func (m *mockStore) CopyObject(ctx context.Context, src, dest string) error              { return nil }
func (m *mockStore) MoveObject(ctx context.Context, src, dest string) error              { return nil }
func (m *mockStore) PresignGet(ctx context.Context, name string, exp time.Duration) (string, error) {
	return "", nil
}
func (m *mockStore) PresignPut(ctx context.Context, name string, exp time.Duration) (string, error) {
	return "", nil
}
func (m *mockStore) CleanUp() error                        { return nil }
func (m *mockStore) HealthCheck(ctx context.Context) error { return nil }

func TestRegisterAndNew(t *testing.T) {
	Register("mock", func(config any) (Store, error) {
		return &mockStore{}, nil
	})

	t.Run("Valid Driver", func(t *testing.T) {
		s, err := New(mockConfig{name: "mock"})
		if err != nil {
			t.Fatalf("Expected nil error, got %v", err)
		}
		if s == nil {
			t.Fatal("Expected store, got nil")
		}
	})

	t.Run("Unknown Driver", func(t *testing.T) {
		_, err := New(mockConfig{name: "unknown"})
		if err != ErrUnknownDriver {
			t.Fatalf("Expected ErrUnknownDriver, got %v", err)
		}
	})
}

func TestScheme(t *testing.T) {
	if Scheme(true) != "https" {
		t.Error("Expected https for useSSL=true")
	}
	if Scheme(false) != "http" {
		t.Error("Expected http for useSSL=false")
	}
}

func TestSplitPath(t *testing.T) {
	t.Run("With currentBucket", func(t *testing.T) {
		b, o, err := SplitPath("b1", "o1")
		if err != nil || b != "b1" || o != "o1" {
			t.Errorf("Unexpected results: %v, %v, %v", b, o, err)
		}
	})

	t.Run("With currentBucket empty path", func(t *testing.T) {
		_, _, err := SplitPath("b1", "")
		if err != ErrInvalidObjectName {
			t.Errorf("Expected ErrInvalidObjectName, got %v", err)
		}
	})

	t.Run("Without currentBucket valid path", func(t *testing.T) {
		b, o, err := SplitPath("", "b1/o1")
		if err != nil || b != "b1" || o != "o1" {
			t.Errorf("Unexpected results: %v, %v, %v", b, o, err)
		}
	})

	t.Run("Without currentBucket invalid path", func(t *testing.T) {
		_, _, err := SplitPath("", "invalid")
		if err != ErrInvalidObjectName {
			t.Errorf("Expected ErrInvalidObjectName, got %v", err)
		}
	})

	t.Run("Without currentBucket empty bucket part", func(t *testing.T) {
		_, _, err := SplitPath("", "/o1")
		if err != ErrInvalidObjectName {
			t.Errorf("Expected ErrInvalidObjectName, got %v", err)
		}
	})

	t.Run("Without currentBucket empty object part", func(t *testing.T) {
		_, _, err := SplitPath("", "b1/")
		if err != ErrInvalidObjectName {
			t.Errorf("Expected ErrInvalidObjectName, got %v", err)
		}
	})
}

func TestGetStreamSize(t *testing.T) {
	t.Run("From strings.Reader (Seeker)", func(t *testing.T) {
		content := "hello world"
		reader := strings.NewReader(content)
		r, size, err := GetStreamSize(reader)
		if err != nil || size != int64(len(content)) {
			t.Errorf("Unexpected results: %v, %v, %v", r, size, err)
		}
	})

	t.Run("From Generic Reader (Fallback)", func(t *testing.T) {
		content := "hello world"
		// wrap strings.Reader to hide Seek method
		reader := struct{ io.Reader }{strings.NewReader(content)}
		r, size, err := GetStreamSize(reader)
		if err != nil || size != int64(len(content)) {
			t.Errorf("Unexpected results: %v, %v, %v", r, size, err)
		}
	})

	t.Run("From *os.File", func(t *testing.T) {
		f, err := os.CreateTemp("", "test-file")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(f.Name())
		defer f.Close()

		content := "file content"
		if _, err := f.WriteString(content); err != nil {
			t.Fatal(err)
		}
		if _, err := f.Seek(0, 0); err != nil {
			t.Fatal(err)
		}

		_, size, err := GetStreamSize(f)
		if err != nil || size != int64(len(content)) {
			t.Errorf("Unexpected results: size=%v, err=%v", size, err)
		}
	})

	t.Run("Seek error", func(t *testing.T) {
		reader := errorSeeker{strings.NewReader("hello")}
		_, _, err := GetStreamSize(reader)
		if err == nil || err.Error() != "seek error" {
			t.Errorf("Expected seek error, got %v", err)
		}
	})

	t.Run("Seek end error", func(t *testing.T) {
		reader := &errorSeekerEnd{strings.NewReader("hello")}
		_, _, err := GetStreamSize(reader)
		if err == nil || err.Error() != "seek end error" {
			t.Errorf("Expected seek end error, got %v", err)
		}
	})

	t.Run("Seek start error", func(t *testing.T) {
		reader := &errorSeekerStart{Reader: strings.NewReader("hello")}
		_, _, err := GetStreamSize(reader)
		if err == nil || err.Error() != "seek start error" {
			t.Errorf("Expected seek start error, got %v", err)
		}
	})

	t.Run("Stat error", func(t *testing.T) {
		reader := &errorStater{Reader: strings.NewReader("hello")}
		_, size, err := GetStreamSize(reader)
		// Should fall back to Seeker since it's also a Seeker
		if err != nil || size != 5 {
			t.Errorf("Expected success via fallback to seeker, got size=%v, err=%v", size, err)
		}
	})

	t.Run("Stat error no fallback", func(t *testing.T) {
		reader := &errorStaterOnly{}
		_, _, err := GetStreamSize(reader)
		if err == nil || err.Error() != "read error" {
			t.Errorf("Expected read error from fallback, got %v", err)
		}
	})

	t.Run("Copy error", func(t *testing.T) {
		reader := &errorReader{}
		_, _, err := GetStreamSize(reader)
		if err == nil || err.Error() != "read error" {
			t.Errorf("Expected read error, got %v", err)
		}
	})
}

type errorSeeker struct {
	io.Reader
}

func (e errorSeeker) Seek(offset int64, whence int) (int64, error) {
	return 0, errors.New("seek error")
}

type errorSeekerEnd struct {
	*strings.Reader
}

func (e *errorSeekerEnd) Seek(offset int64, whence int) (int64, error) {
	if whence == io.SeekEnd {
		return 0, errors.New("seek end error")
	}
	return e.Reader.Seek(offset, whence)
}

type errorSeekerStart struct {
	*strings.Reader
}

func (e *errorSeekerStart) Seek(offset int64, whence int) (int64, error) {
	if whence == io.SeekStart {
		return 0, errors.New("seek start error")
	}
	return e.Reader.Seek(offset, whence)
}

type errorStater struct {
	*strings.Reader
}

func (e *errorStater) Stat() (os.FileInfo, error) {
	return nil, errors.New("stat error")
}

type errorStaterOnly struct{}

func (e *errorStaterOnly) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}

func (e *errorStaterOnly) Stat() (os.FileInfo, error) {
	return nil, errors.New("stat error")
}

type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}
