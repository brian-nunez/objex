package filesystem

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/brian-nunez/objex"
)

var (
	osMkdirAll = os.MkdirAll
	osCreate   = os.Create
	osOpen     = os.Open
	osStat     = os.Stat
	osRemove   = os.Remove
	osRemoveAll = os.RemoveAll
	osReadDir  = os.ReadDir
	filepathWalkDir = filepath.WalkDir
	ioCopy = io.Copy
	urlParse = url.Parse
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
	BasePath  string
	BaseURL   string
	SecretKey string
}

func (c Config) DriverName() string {
	return driverName
}

type Store struct {
	basePath  string
	baseURL   string
	secretKey string
	bucket    string
}

func NewStore(config Config) (*Store, error) {
	if config.BasePath == "" {
		return nil, objex.ErrInvalidEndpoint
	}
	return &Store{
		basePath:  config.BasePath,
		baseURL:   config.BaseURL,
		secretKey: config.SecretKey,
	}, nil
}

func (s *Store) Setup(ctx context.Context) error {
	return osMkdirAll(s.basePath, 0755)
}

func (s *Store) SetBucket(bucketName string) (bool, error) {
	path := filepath.Join(s.basePath, bucketName)
	err := osMkdirAll(path, 0755)
	if err != nil {
		return false, err
	}
	s.bucket = bucketName
	return true, nil
}

func (s *Store) SetRegion(region string) error {
	return nil
}

func (s *Store) CreateBucket(ctx context.Context, bucketName string) error {
	return osMkdirAll(filepath.Join(s.basePath, bucketName), 0755)
}

func (s *Store) DeleteBucket(ctx context.Context, bucketName string) error {
	return osRemoveAll(filepath.Join(s.basePath, bucketName))
}

func (s *Store) ListBuckets(ctx context.Context) ([]objex.Bucket, error) {
	entries, err := osReadDir(s.basePath)
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

func (s *Store) CreateObject(ctx context.Context, name string, data io.Reader, contentType string) (string, error) {
	bucket, object, err := splitPathFS(s.bucket, name)
	if err != nil {
		return "", err
	}

	fullPath := filepath.Join(s.basePath, bucket, object)
	err = osMkdirAll(filepath.Dir(fullPath), 0755)
	if err != nil {
		return "", err
	}

	outFile, err := osCreate(fullPath)
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	hash := md5.New()
	mw := io.MultiWriter(outFile, hash)

	_, err = ioCopy(mw, data)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (s *Store) ReadObject(ctx context.Context, name string) (io.ReadCloser, error) {
	bucket, object, err := splitPathFS(s.bucket, name)
	if err != nil {
		return nil, err
	}
	return osOpen(filepath.Join(s.basePath, bucket, object))
}

func (s *Store) ReadObjectRange(ctx context.Context, name string, offset, length int64) (io.ReadCloser, error) {
	bucket, object, err := splitPathFS(s.bucket, name)
	if err != nil {
		return nil, err
	}
	
	f, err := osOpen(filepath.Join(s.basePath, bucket, object))
	if err != nil {
		return nil, err
	}

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		f.Close()
		return nil, err
	}

	return &limitedReadCloser{R: io.LimitReader(f, length), C: f}, nil
}

type limitedReadCloser struct {
	R io.Reader
	C io.Closer
}

func (l *limitedReadCloser) Read(p []byte) (n int, err error) {
	return l.R.Read(p)
}

func (l *limitedReadCloser) Close() error {
	return l.C.Close()
}

func (s *Store) UpdateObject(ctx context.Context, name string, data io.Reader) (string, error) {
	return s.CreateObject(ctx, name, data, "")
}

func (s *Store) DeleteObject(ctx context.Context, name string) error {
	bucket, object, err := splitPathFS(s.bucket, name)
	if err != nil {
		return err
	}
	return osRemove(filepath.Join(s.basePath, bucket, object))
}

func (s *Store) ListObjects(ctx context.Context, bucket string) ([]*objex.ObjectMetaData, error) {
	if bucket == "" {
		bucket = s.bucket
	}

	var objects []*objex.ObjectMetaData
	base := filepath.Join(s.basePath, bucket)

	err := filepathWalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil || d.IsDir() {
			return err
		}
		info, _ := d.Info()
		relative, _ := filepath.Rel(base, path)

		// Note: We don't calculate MD5 during list for performance reasons.
		// If the user needs the ETag, they should call Metadata() for that specific object.
		objects = append(objects, &objex.ObjectMetaData{
			Key:          relative,
			Size:         info.Size(),
			ContentType:  "application/octet-stream",
			ETag:         "", 
			LastModified: info.ModTime().Format(time.RFC3339),
		})
		return nil
	})
	return objects, err
}

func (s *Store) Exists(ctx context.Context, name string) (bool, *objex.ObjectMetaData, error) {
	bucket, object, err := splitPathFS(s.bucket, name)
	if err != nil {
		return false, nil, err
	}
	path := filepath.Join(s.basePath, bucket, object)
	info, err := osStat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}

	// Calculate MD5 for ETag consistency
	// Warning: This can be slow for very large files.
	etag, _ := s.calculateMD5(path)

	return true, &objex.ObjectMetaData{
		Key:          object,
		Size:         info.Size(),
		LastModified: info.ModTime().Format(time.RFC3339),
		ContentType:  "application/octet-stream",
		ETag:         etag,
	}, nil
}

func (s *Store) Metadata(ctx context.Context, name string) (*objex.ObjectMetaData, error) {
	found, meta, err := s.Exists(ctx, name)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, objex.ErrObjectNotFound
	}
	return meta, nil
}

func (s *Store) calculateMD5(path string) (string, error) {
	f, err := osOpen(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hash := md5.New()
	if _, err := ioCopy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (s *Store) CopyObject(ctx context.Context, src, dest string) error {
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

	err = osMkdirAll(filepath.Dir(destPath), 0755)
	if err != nil {
		return err
	}

	srcFile, err := osOpen(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := osCreate(destPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = ioCopy(destFile, srcFile)
	return err
}

func (s *Store) MoveObject(ctx context.Context, src, dest string) error {
	err := s.CopyObject(ctx, src, dest)
	if err != nil {
		return err
	}
	return s.DeleteObject(ctx, src)
}

func (s *Store) PresignGet(ctx context.Context, name string, expiration time.Duration) (string, error) {
	return s.generatePresignedURL("GET", name, expiration)
}

func (s *Store) PresignPut(ctx context.Context, name string, expiration time.Duration) (string, error) {
	return s.generatePresignedURL("PUT", name, expiration)
}

func (s *Store) generatePresignedURL(method, name string, expiration time.Duration) (string, error) {
	if s.baseURL == "" {
		return "", errors.New("BaseURL not configured for filesystem driver")
	}

	bucket, object, err := splitPathFS(s.bucket, name)
	if err != nil {
		return "", err
	}

	expires := time.Now().Add(expiration).Unix()
	rawURL := fmt.Sprintf("%s/%s/%s", strings.TrimSuffix(s.baseURL, "/"), bucket, object)
	
	u, err := urlParse(rawURL)
	if err != nil {
		return "", err
	}

	q := u.Query()
	q.Set("method", method)
	q.Set("expires", strconv.FormatInt(expires, 10))

	if s.secretKey != "" {
		mac := hmac.New(sha256.New, []byte(s.secretKey))
		msg := fmt.Sprintf("%s:%s:%d", method, u.Path, expires)
		mac.Write([]byte(msg))
		signature := hex.EncodeToString(mac.Sum(nil))
		q.Set("signature", signature)
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (s *Store) CleanUp() error {
	return nil
}

func (s *Store) HealthCheck(ctx context.Context) error {
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

	return ".", name, nil
}
