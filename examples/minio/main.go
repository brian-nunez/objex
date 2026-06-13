package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/brian-nunez/objex/drivers/minio"
)

func main() {
	ctx := context.Background()

	// Configure the Minio store
	store, err := minio.NewStore(minio.Config{
		Endpoint:  "localhost:9000",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		UseSSL:    false,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Set a bucket
	if _, err := store.SetBucket("my-bucket"); err != nil {
		log.Fatal(err)
	}

	// Create an object
	content := "Hello from Minio!"
	etag, err := store.CreateObject(ctx, "hello-minio.txt", strings.NewReader(content), "text/plain")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Created object in Minio with ETag: %s\n", etag)

	// Read the object
	reader, err := store.ReadObject(ctx, "hello-minio.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Read content from Minio: %s\n", string(data))
}
