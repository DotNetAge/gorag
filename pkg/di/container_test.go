package di

import (
	"fmt"
	"testing"
)

// TestInterface represents a test interface
type TestInterface interface {
	GetValue() int
}

// TestImplementation represents a test implementation
type TestImplementation struct {
	value int
}

// GetValue returns the value
func (t *TestImplementation) GetValue() int {
	return t.value
}

func TestContainer_RegisterAndResolve(t *testing.T) {
	c := New()
	
	// Register a singleton service
	c.Register((*TestInterface)(nil), func(c *Container) (interface{}, error) {
		return &TestImplementation{value: 42}, nil
	}, true)
	
	// Resolve the service
	instance, err := c.Resolve((*TestInterface)(nil))
	if err != nil {
		t.Fatalf("Failed to resolve service: %v", err)
	}
	
	testInstance, ok := instance.(TestInterface)
	if !ok {
		t.Fatalf("Resolved instance is not of type TestInterface")
	}
	
	if testInstance.GetValue() != 42 {
		t.Fatalf("Expected value 42, got %d", testInstance.GetValue())
	}
}

func TestContainer_RegisterInstance(t *testing.T) {
	c := New()
	
	// Create an instance
	instance := &TestImplementation{value: 100}
	
	// Register the instance
	c.RegisterInstance((*TestInterface)(nil), instance)
	
	// Resolve the service
	resolvedInstance, err := c.Resolve((*TestInterface)(nil))
	if err != nil {
		t.Fatalf("Failed to resolve service: %v", err)
	}
	
	if resolvedInstance != instance {
		t.Fatalf("Resolved instance is not the same as registered instance")
	}
}

func TestContainer_TransientService(t *testing.T) {
	c := New()
	
	// Register a transient service
	c.Register((*TestInterface)(nil), func(c *Container) (interface{}, error) {
		return &TestImplementation{value: 42}, nil
	}, false)
	
	// Resolve the service twice
	instance1, err := c.Resolve((*TestInterface)(nil))
	if err != nil {
		t.Fatalf("Failed to resolve service: %v", err)
	}
	
	instance2, err := c.Resolve((*TestInterface)(nil))
	if err != nil {
		t.Fatalf("Failed to resolve service: %v", err)
	}
	
	// They should be different instances
	if instance1 == instance2 {
		t.Fatalf("Transient services should return different instances")
	}
}

func TestContainer_SingletonService(t *testing.T) {
	c := New()
	
	// Register a singleton service
	c.Register((*TestInterface)(nil), func(c *Container) (interface{}, error) {
		return &TestImplementation{value: 42}, nil
	}, true)
	
	// Resolve the service twice
	instance1, err := c.Resolve((*TestInterface)(nil))
	if err != nil {
		t.Fatalf("Failed to resolve service: %v", err)
	}
	
	instance2, err := c.Resolve((*TestInterface)(nil))
	if err != nil {
		t.Fatalf("Failed to resolve service: %v", err)
	}
	
	// They should be the same instance
	if instance1 != instance2 {
		t.Fatalf("Singleton services should return the same instance")
	}
}

func TestContainer_MustResolve(t *testing.T) {
	c := New()
	
	// Register a service
	c.Register((*TestInterface)(nil), func(c *Container) (interface{}, error) {
		return &TestImplementation{value: 42}, nil
	}, true)
	
	// MustResolve should work
	instance := c.MustResolve((*TestInterface)(nil))
	testInstance, ok := instance.(TestInterface)
	if !ok {
		t.Fatalf("Resolved instance is not of type TestInterface")
	}
	
	if testInstance.GetValue() != 42 {
		t.Fatalf("Expected value 42, got %d", testInstance.GetValue())
	}
}

func TestContainer_IsRegistered(t *testing.T) {
	c := New()
	
	// Check if a service is registered
	if c.IsRegistered((*TestInterface)(nil)) {
		t.Fatalf("Service should not be registered yet")
	}
	
	// Register a service
	c.Register((*TestInterface)(nil), func(c *Container) (interface{}, error) {
		return &TestImplementation{value: 42}, nil
	}, true)
	
	// Check if the service is registered
	if !c.IsRegistered((*TestInterface)(nil)) {
		t.Fatalf("Service should be registered")
	}
}

func TestContainer_Clear(t *testing.T) {
	c := New()
	
	// Register a service
	c.Register((*TestInterface)(nil), func(c *Container) (interface{}, error) {
		return &TestImplementation{value: 42}, nil
	}, true)
	
	// Clear the container
	c.Clear()
	
	// Check if the service is no longer registered
	if c.IsRegistered((*TestInterface)(nil)) {
		t.Fatalf("Service should not be registered after Clear")
	}
}

func TestContainer_Register_NilInterface(t *testing.T) {
	c := New()
	
	// Should panic when registering with nil interface
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Expected panic when registering with nil interface")
		}
	}()
	
	c.Register(nil, func(c *Container) (interface{}, error) {
		return &TestImplementation{value: 42}, nil
	}, true)
}

func TestContainer_Register_NonInterface(t *testing.T) {
	c := New()
	
	// Should panic when registering with non-interface type
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Expected panic when registering with non-interface type")
		}
	}()
	
	c.Register(42, func(c *Container) (interface{}, error) {
		return &TestImplementation{value: 42}, nil
	}, true)
}

func TestContainer_RegisterInstance_NilInterface(t *testing.T) {
	c := New()
	
	// Should panic when registering instance with nil interface
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Expected panic when registering instance with nil interface")
		}
	}()
	
	c.RegisterInstance(nil, &TestImplementation{value: 42})
}

func TestContainer_RegisterInstance_NonInterface(t *testing.T) {
	c := New()
	
	// Should panic when registering instance with non-interface type
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Expected panic when registering instance with non-interface type")
		}
	}()
	
	c.RegisterInstance(42, &TestImplementation{value: 42})
}

func TestContainer_RegisterInstance_InvalidInstance(t *testing.T) {
	c := New()
	
	// Should panic when registering instance that doesn't implement interface
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Expected panic when registering instance that doesn't implement interface")
		}
	}()
	
	c.RegisterInstance((*TestInterface)(nil), 42)
}

func TestContainer_Resolve_NilInterface(t *testing.T) {
	c := New()
	
	// Should return error when resolving with nil interface
	_, err := c.Resolve(nil)
	if err == nil {
		t.Fatalf("Expected error when resolving with nil interface")
	}
}

func TestContainer_Resolve_NonInterface(t *testing.T) {
	c := New()
	
	// Should return error when resolving with non-interface type
	_, err := c.Resolve(42)
	if err == nil {
		t.Fatalf("Expected error when resolving with non-interface type")
	}
}

func TestContainer_Resolve_UnregisteredService(t *testing.T) {
	c := New()
	
	// Should return error when resolving unregistered service
	_, err := c.Resolve((*TestInterface)(nil))
	if err == nil {
		t.Fatalf("Expected error when resolving unregistered service")
	}
}

func TestContainer_Resolve_FactoryError(t *testing.T) {
	c := New()
	
	// Register a service with a factory that returns an error
	c.Register((*TestInterface)(nil), func(c *Container) (interface{}, error) {
		return nil, fmt.Errorf("factory error")
	}, true)
	
	// Should return error when resolving
	_, err := c.Resolve((*TestInterface)(nil))
	if err == nil {
		t.Fatalf("Expected error when factory returns error")
	}
}

func TestContainer_MustResolve_Panic(t *testing.T) {
	c := New()
	
	// Should panic when resolving unregistered service
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Expected panic when MustResolve fails")
		}
	}()
	
	c.MustResolve((*TestInterface)(nil))
}

func TestContainer_IsRegistered_NilInterface(t *testing.T) {
	c := New()
	
	// Should return false when checking nil interface
	if c.IsRegistered(nil) {
		t.Fatalf("Expected false when checking nil interface")
	}
}

func TestContainer_IsRegistered_NonInterface(t *testing.T) {
	c := New()
	
	// Should return false when checking non-interface type
	if c.IsRegistered(42) {
		t.Fatalf("Expected false when checking non-interface type")
	}
}
