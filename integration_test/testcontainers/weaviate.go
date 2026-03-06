package testcontainers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// WeaviateContainer represents a Weaviate test container
type WeaviateContainer struct {
	testcontainers.Container
	Host string
	Port string
}

// NewWeaviateContainer creates a new Weaviate test container
func NewWeaviateContainer(t *testing.T) (*WeaviateContainer, error) {
	ctx := context.Background()
	
	req := testcontainers.ContainerRequest{
		Image:        "semitechnologies/weaviate:1.20.0",
		ExposedPorts: []string{"8080/tcp"},
		WaitingFor:   wait.ForHTTP("/v1/meta").WithStartupTimeout(1 * time.Minute),
		Env: map[string]string{
			"AUTHENTICATION_ANONYMOUS_ACCESS_ENABLED": "true",
			"PERSISTENCE_DATA_PATH":                  "/var/lib/weaviate",
		},
	}
	
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start Weaviate container: %w", err)
	}
	
	host, err := container.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get container host: %w", err)
	}
	
	port, err := container.MappedPort(ctx, "8080/tcp")
	if err != nil {
		return nil, fmt.Errorf("failed to get container port: %w", err)
	}
	
	return &WeaviateContainer{
		Container: container,
		Host:      host,
		Port:      port.Port(),
	}, nil
}
