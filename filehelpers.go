package objex

import (
	"bytes"
	"io"
	"os"
	"strings"
)

func Scheme(useSSL bool) string {
	if useSSL {
		return "https"
	}
	return "http"
}

func SplitPath(currentBucket string, fullPath string) (bucket, object string, err error) {
	if currentBucket != "" {
		if fullPath == "" {
			return "", "", ErrInvalidObjectName
		}
		return currentBucket, fullPath, nil
	}

	parts := strings.SplitN(fullPath, "/", 2)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", ErrInvalidObjectName
	}

	return parts[0], parts[1], nil
}

func GetStreamSize(data io.Reader) (io.Reader, int64, error) {
	// If it has a Stat() method (like *os.File), use it
	if s, ok := data.(interface{ Stat() (os.FileInfo, error) }); ok {
		info, err := s.Stat()
		if err == nil {
			return data, info.Size(), nil
		}
		// If Stat() fails, we might still be able to use Seek or fallback
	}

	// If it's a seeker, find size without buffering
	if seeker, ok := data.(io.ReadSeeker); ok {
		currentPos, err := seeker.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, 0, err
		}
		end, err := seeker.Seek(0, io.SeekEnd)
		if err != nil {
			return nil, 0, err
		}
		size := end - currentPos
		_, err = seeker.Seek(currentPos, io.SeekStart)
		if err != nil {
			return nil, 0, err
		}
		return seeker, size, nil
	}

	// Fallback: buffer to memory (Only for small files, but we don't know the size yet)
	// In a real-world scenario, we might want to use a temporary file for large streams
	// if the driver requires a size upfront.
	var buf bytes.Buffer
	n, err := io.Copy(&buf, data)
	if err != nil {
		return nil, 0, err
	}

	return bytes.NewReader(buf.Bytes()), n, nil
}
