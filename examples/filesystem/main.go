package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/brian-nunez/objex/drivers/filesystem"
)

func main() {
	ctx := context.Background()

	// Configure the filesystem store
	store, err := filesystem.NewStore(filesystem.Config{
		BasePath:  "./tmp-storage",
		BaseURL:   "http://localhost:8080",
		SecretKey: "super-secret",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Ensure the base path exists
	if err := store.Setup(ctx); err != nil {
		log.Fatal(err)
	}

	// Set a bucket (creates directory)
	if _, err := store.SetBucket("my-bucket"); err != nil {
		log.Fatal(err)
	}

	// Create an object
	content := "Hello, Objex!"
	etag, err := store.CreateObject(ctx, "hello.txt", strings.NewReader(content), "text/plain")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Created object with ETag: %s\n", etag)

	// Read the object
	reader, err := store.ReadObject(ctx, "hello.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Read content: %s\n", string(data))

	// Generate a presigned URL
	url, err := store.PresignGet(ctx, "hello.txt", time.Hour)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Presigned GET URL: %s\n", url)

	// Cleanup
	defer os.RemoveAll("./tmp-storage")
}
