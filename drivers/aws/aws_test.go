package aws

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestAWSConfig(t *testing.T) {
	config := Config{
		Region: "us-east-1",
	}
	if config.DriverName() != "aws" {
		t.Error("Wrong driver name")
	}
}

func TestAWSInit(t *testing.T) {
	t.Run("DriverName", func(t *testing.T) {
		s := &Store{}
		if s.DriverName() != "aws" {
			t.Error("Wrong driver name")
		}
	})

	t.Run("CleanUp", func(t *testing.T) {
		s := &Store{}
		if err := s.CleanUp(); err != nil {
			t.Errorf("CleanUp failed: %v", err)
		}
	})

	t.Run("Setup", func(t *testing.T) {
		s := &Store{}
		if err := s.Setup(context.Background()); err != nil {
			t.Errorf("Setup failed: %v", err)
		}
	})

	t.Run("HealthCheck", func(t *testing.T) {
		s := &Store{client: &mockS3{listBuckets: func(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
			return &s3.ListBucketsOutput{}, nil
		}}}
		if err := s.HealthCheck(context.Background()); err != nil {
			t.Errorf("HealthCheck failed: %v", err)
		}
	})
}
