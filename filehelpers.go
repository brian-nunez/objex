package objex

import (
	"bytes"
	"io"
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
		return currentBucket, fullPath, nil
	}

	parts := strings.SplitN(fullPath, "/", 2)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", ErrInvalidObjectName
	}

	return parts[0], parts[1], nil
}

func GetStreamSize(data io.Reader) (io.Reader, int64, error) {
	if seeker, ok := data.(interface {
		io.Reader
		io.Seeker
	}); ok {
		currentPos, _ := seeker.Seek(0, io.SeekCurrent)
		end, err := seeker.Seek(0, io.SeekEnd)
		if err != nil {
			return nil, 0, ErrInvalidFile
		}
		size := end - currentPos
		_, _ = seeker.Seek(currentPos, io.SeekStart)
		return seeker, size, nil
	}

	// Fallback: buffer to memory
	var buf bytes.Buffer
	n, err := io.Copy(&buf, data)
	if err != nil {
		return nil, 0, err
	}

	return bytes.NewReader(buf.Bytes()), n, nil
}
