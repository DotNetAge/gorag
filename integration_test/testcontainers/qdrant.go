package testcontainers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// QdrantContainer represents a Qdrant test container
type QdrantContainer struct {
	testcontainers.Container
	Host     string
	GRPCPort string
	HTTPPort string
}

// NewQdrantContainer creates a new Qdrant test container
func NewQdrantContainer(t *testing.T) (*QdrantContainer, error) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "qdrant/qdrant:v1.12.0",
		ExposedPorts: []string{"6333/tcp", "6334/tcp"},
		WaitingFor:   wait.ForHTTP("/collections").WithPort("6333/tcp").WithStartupTimeout(1 * time.Minute),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start Qdrant container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get container host: %w", err)
	}

	httpPort, err := container.MappedPort(ctx, "6333/tcp")
	if err != nil {
		return nil, fmt.Errorf("failed to get container HTTP port: %w", err)
	}

	grpcPort, err := container.MappedPort(ctx, "6334/tcp")
	if err != nil {
		return nil, fmt.Errorf("failed to get container gRPC port: %w", err)
	}

	return &QdrantContainer{
		Container: container,
		Host:      host,
		HTTPPort:  httpPort.Port(),
		GRPCPort:  grpcPort.Port(),
	}, nil
}
