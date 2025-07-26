package filesystem

import (
	"errors"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brian-nunez/objex"
)

const driverName = "filesystem"

func init() {
	objex.Register(driverName, func(config any) (objex.Store, error) {
		conf, ok := config.(Config)
		if !ok {
			return nil, objex.ErrClientInit
		}
		return NewStore(conf)
	})
}

type Config struct {
	BasePath string
}

func (c Config) DriverName() string {
	return driverName
}

type Store struct {
	basePath string
	bucket   string
}

func NewStore(config Config) (*Store, error) {
	if config.BasePath == "" {
		return nil, objex.ErrInvalidEndpoint
	}
	return &Store{
		basePath: config.BasePath,
	}, nil
}

func (s *Store) Setup() error {
	return os.MkdirAll(s.basePath, 0755)
}

func (s *Store) SetBucket(bucketName string) (bool, error) {
	path := filepath.Join(s.basePath, bucketName)
	err := os.MkdirAll(path, 0755)
	if err != nil {
		return false, err
	}
	s.bucket = bucketName
	return true, nil
}

func (s *Store) SetRegion(region string) error {
	// Not applicable for filesystem
	return nil
}

func (s *Store) CreateBucket(bucketName string) error {
	return os.MkdirAll(filepath.Join(s.basePath, bucketName), 0755)
}

func (s *Store) DeleteBucket(bucketName string) error {
	return os.RemoveAll(filepath.Join(s.basePath, bucketName))
}

func (s *Store) ListBuckets() ([]objex.Bucket, error) {
	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		return nil, err
	}

	var buckets []objex.Bucket
	for _, entry := range entries {
		if entry.IsDir() {
			info, _ := entry.Info()
			buckets = append(buckets, objex.Bucket{
				Name:         entry.Name(),
				CreationDate: info.ModTime().Format(time.RFC3339),
			})
		}
	}
	return buckets, nil
}

func (s *Store) CreateObject(name string, data io.Reader, contentType string) error {
	bucket, object, err := splitPathFS(s.bucket, name)
	log.Printf("[%v] [%v] [%v]", bucket, object, name)
	if err != nil {
		return err
	}

	fullPath := filepath.Join(s.basePath, bucket, object)
	err = os.MkdirAll(filepath.Dir(fullPath), 0755)
	if err != nil {
		return err
	}

	outFile, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, data)
	return err
}

func (s *Store) ReadObject(name string) ([]byte, error) {
	bucket, object, err := splitPathFS(s.bucket, name)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(s.basePath, bucket, object))
}

func (s *Store) UpdateObject(name string, data io.Reader) error {
	return s.CreateObject(name, data, "")
}

func (s *Store) DeleteObject(name string) error {
	bucket, object, err := splitPathFS(s.bucket, name)
	if err != nil {
		return err
	}
	return os.Remove(filepath.Join(s.basePath, bucket, object))
}

func (s *Store) ListObjects(bucket string) ([]*objex.ObjectMetaData, error) {
	if bucket == "" {
		bucket = s.bucket
	}

	var objects []*objex.ObjectMetaData
	base := filepath.Join(s.basePath, bucket)

	err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, _ := d.Info()
		relative, _ := filepath.Rel(base, path)

		objects = append(objects, &objex.ObjectMetaData{
			Key:          relative,
			Size:         info.Size(),
			ContentType:  "application/octet-stream", // Simplified
			ETag:         "",                         // Not used
			LastModified: info.ModTime().Format(time.RFC3339),
		})
		return nil
	})
	return objects, err
}

func (s *Store) Exists(name string) (bool, *objex.ObjectMetaData, error) {
	bucket, object, err := splitPathFS(s.bucket, name)
	if err != nil {
		return false, nil, err
	}
	path := filepath.Join(s.basePath, bucket, object)
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}
	return true, &objex.ObjectMetaData{
		Key:          object,
		Size:         info.Size(),
		LastModified: info.ModTime().Format(time.RFC3339),
		ContentType:  "application/octet-stream",
	}, nil
}

func (s *Store) Metadata(name string) (*objex.ObjectMetaData, error) {
	found, meta, err := s.Exists(name)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, objex.ErrObjectNotFound
	}
	return meta, nil
}

func (s *Store) CopyObject(src, dest string) error {
	srcBucket, srcObject, err := splitPathFS(s.bucket, src)
	if err != nil {
		return err
	}
	destBucket, destObject, err := splitPathFS(s.bucket, dest)
	if err != nil {
		return err
	}

	srcPath := filepath.Join(s.basePath, srcBucket, srcObject)
	destPath := filepath.Join(s.basePath, destBucket, destObject)

	err = os.MkdirAll(filepath.Dir(destPath), 0755)
	if err != nil {
		return err
	}

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	return err
}

func (s *Store) MoveObject(src, dest string) error {
	err := s.CopyObject(src, dest)
	if err != nil {
		return err
	}
	return s.DeleteObject(src)
}

func (s *Store) CleanUp() error {
	log.Println("[Objex Filesystem] CleanUp called — no action needed")
	return nil
}

func (s *Store) HealthCheck() error {
	if s.basePath == "" {
		return objex.ErrInvalidEndpoint
	}
	return os.MkdirAll(s.basePath, 0755)
}

func splitPathFS(bucket, name string) (string, string, error) {
	if name == "" {
		return "", "", objex.ErrInvalidObjectName
	}

	if bucket != "" {
		return bucket, name, nil
	}

	parts := strings.SplitN(name, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1], nil
	}

	// No bucket set, no slash in name — treat as root bucket
	return ".", name, nil
}
