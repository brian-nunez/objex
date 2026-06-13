package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/brian-nunez/objex/drivers/aws"
)

func main() {
	ctx := context.Background()

	// Configure the AWS S3 store
	store, err := aws.NewStore(aws.Config{
		Region:    "us-east-1",
		AccessKey: "YOUR_ACCESS_KEY",
		SecretKey: "YOUR_SECRET_KEY",
		Bucket:    "my-test-bucket",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Create an object
	content := "Hello from AWS S3!"
	etag, err := store.CreateObject(ctx, "hello-aws.txt", strings.NewReader(content), "text/plain")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Created object in S3 with ETag: %s\n", etag)

	// Read the object
	reader, err := store.ReadObject(ctx, "hello-aws.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Read content from S3: %s\n", string(data))
}
