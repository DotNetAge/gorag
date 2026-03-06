package testcontainers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// MilvusContainer represents a Milvus test container
type MilvusContainer struct {
	testcontainers.Container
	Host string
	Port string
}

// NewMilvusContainer creates a new Milvus test container
func NewMilvusContainer(t *testing.T) (*MilvusContainer, error) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "milvusdb/milvus:v2.4.0",
		ExposedPorts: []string{"19530/tcp"},
		WaitingFor:   wait.ForLog("Milvus Proxy successfully initialized and ready to serve").WithStartupTimeout(3 * time.Minute),
		Env: map[string]string{
			"ETCD_USE_EMBED":     "true",
			"COMMON_STORAGETYPE": "local",
		},
		Cmd: []string{"milvus", "run", "standalone"},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start Milvus container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get container host: %w", err)
	}

	port, err := container.MappedPort(ctx, "19530/tcp")
	if err != nil {
		return nil, fmt.Errorf("failed to get container port: %w", err)
	}

	return &MilvusContainer{
		Container: container,
		Host:      host,
		Port:      port.Port(),
	}, nil
}
