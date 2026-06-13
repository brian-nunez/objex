package filesystem

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net/url"
	"os"
	"testing"
	"time"
)

func TestFilesystemMocks(t *testing.T) {
	ctx := context.Background()
	store, _ := NewStore(Config{BasePath: "/tmp"})

	t.Run("Exists Stat Error", func(t *testing.T) {
		oldStat := osStat
		defer func() { osStat = oldStat }()
		osStat = func(name string) (os.FileInfo, error) {
			return nil, errors.New("stat error")
		}

		_, _, err := store.Exists(ctx, "file.txt")
		if err == nil || err.Error() != "stat error" {
			t.Errorf("Expected stat error, got %v", err)
		}
	})

	t.Run("calculateMD5 Open Error", func(t *testing.T) {
		oldOpen := osOpen
		defer func() { osOpen = oldOpen }()
		osOpen = func(name string) (*os.File, error) {
			return nil, errors.New("open error")
		}

		_, err := store.calculateMD5("file.txt")
		if err == nil || err.Error() != "open error" {
			t.Errorf("Expected open error, got %v", err)
		}
	})

	t.Run("ListObjects WalkDir Error", func(t *testing.T) {
		oldWalk := filepathWalkDir
		defer func() { filepathWalkDir = oldWalk }()
		filepathWalkDir = func(root string, fn fs.WalkDirFunc) error {
			return errors.New("walk error")
		}

		_, err := store.ListObjects(ctx, "bucket")
		if err == nil || err.Error() != "walk error" {
			t.Errorf("Expected walk error, got %v", err)
		}
	})

	t.Run("ListObjects WalkDir Callback Error", func(t *testing.T) {
		oldWalk := filepathWalkDir
		defer func() { filepathWalkDir = oldWalk }()
		filepathWalkDir = func(root string, fn fs.WalkDirFunc) error {
			return fn("path", nil, errors.New("callback error"))
		}

		_, err := store.ListObjects(ctx, "bucket")
		if err == nil || err.Error() != "callback error" {
			t.Errorf("Expected callback error, got %v", err)
		}
	})

	t.Run("ioCopy Error", func(t *testing.T) {
		oldCopy := ioCopy
		oldOpen := osOpen
		oldCreate := osCreate
		oldMkdir := osMkdirAll
		defer func() { 
			ioCopy = oldCopy 
			osOpen = oldOpen
			osCreate = oldCreate
			osMkdirAll = oldMkdir
		}()
		
		ioCopy = func(dst io.Writer, src io.Reader) (int64, error) {
			return 0, errors.New("copy error")
		}
		
		tmpFile, _ := os.CreateTemp("", "mock-test")
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		osOpen = func(name string) (*os.File, error) {
			return os.Open(tmpFile.Name())
		}
		osCreate = func(name string) (*os.File, error) {
			return os.Create(tmpFile.Name())
		}
		osMkdirAll = func(path string, perm os.FileMode) error {
			return nil
		}

		_, err := store.CreateObject(ctx, "obj", nil, "")
		if err == nil || err.Error() != "copy error" {
			t.Errorf("Expected copy error in CreateObject, got %v", err)
		}

		_, err = store.calculateMD5("file.txt")
		if err == nil || err.Error() != "copy error" {
			t.Errorf("Expected copy error in calculateMD5, got %v", err)
		}

		err = store.CopyObject(ctx, "src", "dest")
		if err == nil || err.Error() != "copy error" {
			t.Errorf("Expected copy error in CopyObject, got %v", err)
		}
	})

	t.Run("CreateObject MkdirAll Error", func(t *testing.T) {
		oldMkdir := osMkdirAll
		defer func() { osMkdirAll = oldMkdir }()
		osMkdirAll = func(path string, perm os.FileMode) error {
			return errors.New("mkdir error")
		}

		_, err := store.CreateObject(ctx, "bucket/obj", nil, "")
		if err == nil || err.Error() != "mkdir error" {
			t.Errorf("Expected mkdir error, got %v", err)
		}
	})

	t.Run("CreateObject Create Error", func(t *testing.T) {
		oldCreate := osCreate
		defer func() { osCreate = oldCreate }()
		osCreate = func(name string) (*os.File, error) {
			return nil, errors.New("create error")
		}

		_, err := store.CreateObject(ctx, "obj", nil, "")
		if err == nil || err.Error() != "create error" {
			t.Errorf("Expected create error, got %v", err)
		}
	})

	t.Run("ReadObjectRange Open Error", func(t *testing.T) {
		oldOpen := osOpen
		defer func() { osOpen = oldOpen }()
		osOpen = func(name string) (*os.File, error) {
			return nil, errors.New("open error")
		}

		_, err := store.ReadObjectRange(ctx, "obj", 0, 10)
		if err == nil || err.Error() != "open error" {
			t.Errorf("Expected open error, got %v", err)
		}
	})

	t.Run("CopyObject MkdirAll Error", func(t *testing.T) {
		oldMkdir := osMkdirAll
		defer func() { osMkdirAll = oldMkdir }()
		osMkdirAll = func(path string, perm os.FileMode) error {
			return errors.New("mkdir error")
		}

		err := store.CopyObject(ctx, "src", "dest")
		if err == nil || err.Error() != "mkdir error" {
			t.Errorf("Expected mkdir error, got %v", err)
		}
	})

	t.Run("CopyObject Open Error", func(t *testing.T) {
		oldOpen := osOpen
		defer func() { osOpen = oldOpen }()
		osOpen = func(name string) (*os.File, error) {
			return nil, errors.New("open error")
		}

		err := store.CopyObject(ctx, "src", "dest")
		if err == nil || err.Error() != "open error" {
			t.Errorf("Expected open error, got %v", err)
		}
	})

	t.Run("CopyObject Create Error", func(t *testing.T) {
		oldCreate := osCreate
		oldOpen := osOpen
		defer func() { 
			osCreate = oldCreate 
			osOpen = oldOpen
		}()
		
		osOpen = func(name string) (*os.File, error) {
			return os.Open(os.DevNull)
		}
		osCreate = func(name string) (*os.File, error) {
			return nil, errors.New("create error")
		}

		err := store.CopyObject(ctx, "src", "dest")
		if err == nil || err.Error() != "create error" {
			t.Errorf("Expected create error, got %v", err)
		}
	})

	t.Run("generatePresignedURL Parse Error", func(t *testing.T) {
		oldParse := urlParse
		defer func() { urlParse = oldParse }()
		urlParse = func(rawURL string) (*url.URL, error) {
			return nil, errors.New("parse error")
		}

		store2, _ := NewStore(Config{BasePath: "/tmp", BaseURL: "http://localhost"})
		_, err := store2.PresignGet(ctx, "obj", time.Hour)
		if err == nil || err.Error() != "parse error" {
			t.Errorf("Expected parse error, got %v", err)
		}
	})
}
