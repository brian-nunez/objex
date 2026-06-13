package minio

import (
	"context"
	"testing"

	"github.com/brian-nunez/objex"
)

func TestMinioConfig(t *testing.T) {
	config := Config{}
	if config.DriverName() != "minio" {
		t.Error("Wrong driver name")
	}
}

func TestMinioHelpers(t *testing.T) {
	t.Run("ToStandardError", func(t *testing.T) {
		if ToStandardError(nil) != nil {
			t.Error("Expected nil")
		}
		// We can't easily create minio.ErrorResponse directly for all cases without network
		// but we can test the nil and empty cases.
	})
}

func TestMinioStoreBasics(t *testing.T) {
	s := &Store{}
	t.Run("Setup", func(t *testing.T) {
		if err := s.Setup(context.Background()); err != nil {
			t.Error(err)
		}
	})
	t.Run("CleanUp", func(t *testing.T) {
		if err := s.CleanUp(); err != nil {
			t.Error(err)
		}
	})
}

func TestMinioHealthCheck(t *testing.T) {
	t.Run("Invalid Endpoint", func(t *testing.T) {
		s := &Store{config: Config{}}
		if err := s.HealthCheck(context.Background()); err != objex.ErrInvalidEndpoint {
			t.Errorf("Expected ErrInvalidEndpoint, got %v", err)
		}
	})
	t.Run("Invalid AccessKey", func(t *testing.T) {
		s := &Store{config: Config{Endpoint: "e"}}
		if err := s.HealthCheck(context.Background()); err != objex.ErrInvalidAccessKey {
			t.Errorf("Expected ErrInvalidAccessKey, got %v", err)
		}
	})
	t.Run("Invalid SecretKey", func(t *testing.T) {
		s := &Store{config: Config{Endpoint: "e", AccessKey: "a"}}
		if err := s.HealthCheck(context.Background()); err != objex.ErrInvalidSecretKey {
			t.Errorf("Expected ErrInvalidSecretKey, got %v", err)
		}
	})
}
