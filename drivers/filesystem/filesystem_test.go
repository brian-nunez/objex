package filesystem

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brian-nunez/objex"
)

type otherConfig struct{}

func (o otherConfig) DriverName() string { return "filesystem" }

type errorReader struct{}

func (e errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}

func TestFilesystem(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "objex-fs-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	config := Config{
		BasePath:  tempDir,
		BaseURL:   "http://localhost:8080",
		SecretKey: "secret",
	}

	store, err := NewStore(config)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	t.Run("DriverName", func(t *testing.T) {
		if config.DriverName() != "filesystem" {
			t.Error("Wrong driver name")
		}
	})

	t.Run("Setup", func(t *testing.T) {
		if err := store.Setup(ctx); err != nil {
			t.Errorf("Setup failed: %v", err)
		}
	})

	t.Run("Bucket Operations", func(t *testing.T) {
		bucket := "test-bucket"
		found, err := store.SetBucket(bucket)
		if err != nil || !found {
			t.Errorf("SetBucket failed: %v, found=%v", err, found)
		}

		if err := store.CreateBucket(ctx, "another-bucket"); err != nil {
			t.Errorf("CreateBucket failed: %v", err)
		}

		buckets, err := store.ListBuckets(ctx)
		if err != nil || len(buckets) < 2 {
			t.Errorf("ListBuckets failed: %v, len=%v", err, len(buckets))
		}

		if err := store.DeleteBucket(ctx, "another-bucket"); err != nil {
			t.Errorf("DeleteBucket failed: %v", err)
		}
	})

	t.Run("Object Operations", func(t *testing.T) {
		objName := "test-obj.txt"
		content := "hello world"
		
		// Create
		etag, err := store.CreateObject(ctx, objName, strings.NewReader(content), "text/plain")
		if err != nil || etag == "" {
			t.Errorf("CreateObject failed: %v, etag=%s", err, etag)
		}

		// Exists & Metadata
		exists, meta, err := store.Exists(ctx, objName)
		if err != nil || !exists || meta.Size != int64(len(content)) || meta.ETag != etag {
			t.Errorf("Exists failed: %v, exists=%v, meta=%+v", err, exists, meta)
		}

		meta, err = store.Metadata(ctx, objName)
		if err != nil || meta.Key != objName {
			t.Errorf("Metadata failed: %v, meta=%+v", err, meta)
		}

		// Read
		rc, err := store.ReadObject(ctx, objName)
		if err != nil {
			t.Errorf("ReadObject failed: %v", err)
		} else {
			data, _ := io.ReadAll(rc)
			rc.Close()
			if string(data) != content {
				t.Errorf("Content mismatch: expected %s, got %s", content, string(data))
			}
		}

		// Read Range
		rc, err = store.ReadObjectRange(ctx, objName, 0, 5)
		if err != nil {
			t.Errorf("ReadObjectRange failed: %v", err)
		} else {
			data, _ := io.ReadAll(rc)
			rc.Close()
			if string(data) != "hello" {
				t.Errorf("Range content mismatch: expected hello, got %s", string(data))
			}
		}

		// Update
		newContent := "updated content"
		newEtag, err := store.UpdateObject(ctx, objName, strings.NewReader(newContent))
		if err != nil || newEtag == etag {
			t.Errorf("UpdateObject failed: %v, etag unchanged", err)
		}

		// List Objects
		objs, err := store.ListObjects(ctx, "")
		if err != nil || len(objs) == 0 {
			t.Errorf("ListObjects failed: %v", err)
		}

		// Copy & Move
		if err := store.CopyObject(ctx, objName, "copied.txt"); err != nil {
			t.Errorf("CopyObject failed: %v", err)
		}
		if err := store.MoveObject(ctx, "copied.txt", "moved.txt"); err != nil {
			t.Errorf("MoveObject failed: %v", err)
		}

		// Delete
		if err := store.DeleteObject(ctx, objName); err != nil {
			t.Errorf("DeleteObject failed: %v", err)
		}
		if err := store.DeleteObject(ctx, "moved.txt"); err != nil {
			t.Errorf("DeleteObject failed: %v", err)
		}
	})

	t.Run("Presigned URLs", func(t *testing.T) {
		urlGet, err := store.PresignGet(ctx, "test.txt", time.Hour)
		if err != nil || urlGet == "" {
			t.Errorf("PresignGet failed: %v", err)
		}
		if !strings.Contains(urlGet, "signature=") || !strings.Contains(urlGet, "method=GET") {
			t.Errorf("PresignGet URL invalid: %s", urlGet)
		}

		urlPut, err := store.PresignPut(ctx, "test.txt", time.Hour)
		if err != nil || urlPut == "" {
			t.Errorf("PresignPut failed: %v", err)
		}
		if !strings.Contains(urlPut, "method=PUT") {
			t.Errorf("PresignPut URL invalid: %s", urlPut)
		}
	})

	t.Run("HealthCheck", func(t *testing.T) {
		if err := store.HealthCheck(ctx); err != nil {
			t.Errorf("HealthCheck failed: %v", err)
		}
	})

	t.Run("CleanUp", func(t *testing.T) {
		if err := store.CleanUp(); err != nil {
			t.Errorf("CleanUp failed: %v", err)
		}
	})

	t.Run("Objex New Integration", func(t *testing.T) {
		s, err := objex.New(Config{BasePath: tempDir})
		if err != nil {
			t.Fatalf("objex.New failed: %v", err)
		}
		if s == nil {
			t.Fatal("Expected store, got nil")
		}

		// Test invalid config type
		// We need a type that implements DriverName() but is not Config
		_, err = objex.New(otherConfig{})
		if err == nil {
			t.Error("Expected error for invalid config type")
		}
	})

	t.Run("Misc", func(t *testing.T) {
		if err := store.SetRegion("us-east-1"); err != nil {
			t.Errorf("SetRegion failed: %v", err)
		}
	})

	t.Run("Path Helpers", func(t *testing.T) {
		b, o, err := splitPathFS("", "bucket/object")
		if err != nil || b != "bucket" || o != "object" {
			t.Errorf("splitPathFS failed: %v, %s, %s", err, b, o)
		}

		b, o, err = splitPathFS("", "object")
		if err != nil || b != "." || o != "object" {
			t.Errorf("splitPathFS failed: %v, %s, %s", err, b, o)
		}

		_, _, err = splitPathFS("", "")
		if err != objex.ErrInvalidObjectName {
			t.Errorf("Expected ErrInvalidObjectName, got %v", err)
		}
	})
	
	t.Run("Error Cases", func(t *testing.T) {
		// Metadata non-existent
		_, err := store.Metadata(ctx, "non-existent")
		if err != objex.ErrObjectNotFound {
			t.Errorf("Expected ErrObjectNotFound, got %v", err)
		}
		
		// NewStore invalid endpoint
		_, err = NewStore(Config{BasePath: ""})
		if err != objex.ErrInvalidEndpoint {
			t.Errorf("Expected ErrInvalidEndpoint, got %v", err)
		}
		
		// Read non-existent
		_, err = store.ReadObject(ctx, "non-existent")
		if err == nil {
			t.Error("Expected error reading non-existent file")
		}

		// Read Range non-existent
		_, err = store.ReadObjectRange(ctx, "non-existent", 0, 10)
		if err == nil {
			t.Error("Expected error reading range of non-existent file")
		}

		// Delete non-existent (should not return error in many FS implementations, but let's see)
		err = store.DeleteObject(ctx, "non-existent")
		// In our implementation it calls os.Remove which returns error if not exists
		if err == nil {
			t.Error("Expected error deleting non-existent file")
		}

		// Copy non-existent
		err = store.CopyObject(ctx, "non-existent", "dest")
		if err == nil {
			t.Error("Expected error copying non-existent file")
		}

		// Move non-existent
		err = store.MoveObject(ctx, "non-existent", "dest")
		if err == nil {
			t.Error("Expected error moving non-existent file")
		}

		// Create with read error
		_, err = store.CreateObject(ctx, "error.txt", errorReader{}, "text/plain")
		if err == nil || !strings.Contains(err.Error(), "read error") {
			t.Errorf("Expected read error, got %v", err)
		}

		// HealthCheck failure
		s3, _ := NewStore(Config{BasePath: "/non/existent/path/that/cannot/be/created/hopefully"})
		// Actually Setup creates it. HealthCheck just checks if BasePath is empty.
		s3.basePath = "" 
		if err := s3.HealthCheck(ctx); err == nil {
			t.Error("Expected HealthCheck error for empty BasePath")
		}

		// SetBucket error (path is a file)
		file, _ := os.CreateTemp(tempDir, "file")
		file.Close()
		s4, _ := NewStore(Config{BasePath: file.Name()})
		_, err = s4.SetBucket("bucket")
		if err == nil {
			t.Error("Expected SetBucket error when BasePath is a file")
		}

		// ListBuckets error
		s5, _ := NewStore(Config{BasePath: "/proc/nonexistent"}) // Or something that doesn't exist
		_, err = s5.ListBuckets(ctx)
		if err == nil {
			t.Error("Expected ListBuckets error for non-existent path")
		}

		// ReadObjectRange seek error
		// We can't easily trigger f.Seek error on a normal file.
		// But we can try to seek past end if it's restricted or something.
		// Actually, let's skip f.Seek error as it's very hard to trigger on os.File.

		// ListObjects error (bucket doesn't exist)
		_, err = store.ListObjects(ctx, "non-existent-bucket")
		if err == nil {
			t.Error("Expected ListObjects error for non-existent bucket")
		}

		// calculateMD5 open error
		_, err = store.calculateMD5(filepath.Join(tempDir, "non-existent"))
		if err == nil {
			t.Error("Expected calculateMD5 error for non-existent file")
		}

		// CreateObject mkdir error
		s6, _ := NewStore(Config{BasePath: file.Name()})
		_, err = s6.CreateObject(ctx, "bucket/obj", strings.NewReader(""), "")
		if err == nil {
			t.Error("Expected CreateObject mkdir error")
		}

		// CreateObject create error
		err = os.MkdirAll(filepath.Join(tempDir, "readonly-dir"), 0555)
		if err != nil {
			t.Fatal(err)
		}
		s7, _ := NewStore(Config{BasePath: filepath.Join(tempDir, "readonly-dir")})
		_, err = s7.CreateObject(ctx, "obj", strings.NewReader(""), "")
		if err == nil {
			// Note: on some systems root might still be able to write
			t.Log("Warning: CreateObject create error not triggered (might be running as root)")
		}

		// ReadObject open error
		_, err = store.ReadObject(ctx, "non-existent")
		if err == nil {
			t.Error("Expected ReadObject error for non-existent file")
		}

		// ReadObjectRange open error
		_, err = store.ReadObjectRange(ctx, "non-existent", 0, 10)
		if err == nil {
			t.Error("Expected ReadObjectRange error for non-existent file")
		}

		// DeleteObject error
		err = store.DeleteObject(ctx, "non-existent")
		if err == nil {
			t.Error("Expected DeleteObject error for non-existent file")
		}

		// CopyObject open error
		err = store.CopyObject(ctx, "non-existent", "dest")
		if err == nil {
			t.Error("Expected CopyObject error for non-existent source")
		}

		// CopyObject create error
		err = store.CopyObject(ctx, "moved.txt", "readonly-dir/dest")
		if err == nil {
			t.Log("Warning: CopyObject create error not triggered")
		}

		// splitPathFS errors
		_, err = store.ReadObject(ctx, "")
		if err == nil {
			t.Error("Expected error for empty name in ReadObject")
		}
		err = store.DeleteObject(ctx, "")
		if err == nil {
			t.Error("Expected error for empty name in DeleteObject")
		}
		_, _, err = store.Exists(ctx, "")
		if err == nil {
			t.Error("Expected error for empty name in Exists")
		}
		_, err = store.Metadata(ctx, "")
		if err == nil {
			t.Error("Expected error for empty name in Metadata")
		}
		_, err = store.CreateObject(ctx, "", nil, "")
		if err == nil {
			t.Error("Expected error for empty name in CreateObject")
		}

		// ReadObjectRange splitPathFS error
		_, err = store.ReadObjectRange(ctx, "", 0, 10)
		if err == nil {
			t.Error("Expected error for empty name in ReadObjectRange")
		}

		// ListObjects cancelled context
		cancelledCtx, cancel := context.WithCancel(ctx)
		cancel()
		_, err = store.ListObjects(cancelledCtx, "")
		if err == nil {
			t.Error("Expected error for cancelled context in ListObjects")
		}

		// CopyObject splitPathFS errors
		err = store.CopyObject(ctx, "", "dest")
		if err == nil {
			t.Error("Expected error for empty src in CopyObject")
		}
		err = store.CopyObject(ctx, "src", "")
		if err == nil {
			t.Error("Expected error for empty dest in CopyObject")
		}

		// generatePresignedURL splitPathFS error
		_, err = store.PresignGet(ctx, "", time.Hour)
		if err == nil {
			t.Error("Expected error for empty name in PresignGet")
		}

		// generatePresignedURL parse error
		s8, _ := NewStore(Config{BasePath: tempDir, BaseURL: "::%"})
		_, err = s8.PresignGet(ctx, "obj", time.Hour)
		if err == nil {
			t.Error("Expected error for invalid BaseURL in PresignGet")
		}

		// ReadObjectRange seek error
		store.CreateObject(ctx, "seek-test.txt", strings.NewReader("hello"), "")
		_, err = store.ReadObjectRange(ctx, "seek-test.txt", -1, 10)
		if err == nil {
			t.Error("Expected error for negative offset in ReadObjectRange")
		}

		// Presigned without BaseURL
		s9, _ := NewStore(Config{BasePath: tempDir})
		_, err = s9.PresignGet(ctx, "test", time.Hour)
		if err == nil {
			t.Error("Expected error for missing BaseURL in PresignGet")
		}
	})
}
