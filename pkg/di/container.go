// Package di provides a lightweight dependency injection container for managing component lifecycle.
//
// This package supports:
//   - Singleton and transient service lifecycles
//   - Interface-based registration and resolution
//   - Thread-safe concurrent access
//   - Factory functions and instance registration
//   - Automatic cleanup of closable services
//
// Example usage:
//
//	container := di.New()
//	container.Register((*MyInterface)(nil), func(c *di.Container) (any, error) {
//	    return NewMyService(), nil
//	}, true) // true = singleton
//
//	service, err := container.Resolve((*MyInterface)(nil))
package di

import (
	"fmt"
	"reflect"
	"sync"
)

// defaultContainer is the global default container
var defaultContainer *Container
var once sync.Once

// Container represents a dependency injection container.
// It manages service registration, lifecycle, and resolution with thread-safe access.
type Container struct {
	services map[reflect.Type]service
	lock     sync.RWMutex
}

// service represents a registered service within the container.
// It holds the service instance, factory function, and lifecycle metadata.
type service struct {
	instance      any
	factory       func(*Container) (any, error)
	isSingleton   bool
	isInitialized bool
}

// New creates a new dependency injection container.
// The container starts empty and requires explicit service registration before use.
func New() *Container {
	return &Container{
		services: make(map[reflect.Type]service),
	}
}

// Register registers a service with the container.
//
// Parameters:
//   - iface: pointer to interface type (e.g., (*MyInterface)(nil))
//   - factory: function that creates the service instance
//   - isSingleton: if true, only one instance is created and reused; if false, new instance each time
//
// Returns the container for method chaining.
func (c *Container) Register(iface any, factory func(*Container) (any, error), isSingleton bool) *Container {
	c.lock.Lock()
	defer c.lock.Unlock()

	ifaceType := reflect.TypeOf(iface)
	if ifaceType == nil {
		panic("Register expects a non-nil interface type")
	}

	// Handle pointer to interface type (e.g., (*TestInterface)(nil))
	if ifaceType.Kind() == reflect.Ptr && ifaceType.Elem().Kind() == reflect.Interface {
		ifaceType = ifaceType.Elem()
	}

	if ifaceType.Kind() != reflect.Interface {
		panic("Register expects an interface type or pointer to interface type")
	}

	c.services[ifaceType] = service{
		factory:     factory,
		isSingleton: isSingleton,
	}

	return c
}

// RegisterInstance registers an existing instance with the container.
// The instance is treated as a singleton and returned as-is on resolution.
//
// Parameters:
//   - iface: pointer to interface type
//   - instance: the concrete instance to register
//
// Returns the container for method chaining.
func (c *Container) RegisterInstance(iface any, instance any) *Container {
	c.lock.Lock()
	defer c.lock.Unlock()

	ifaceType := reflect.TypeOf(iface)
	if ifaceType == nil {
		panic("RegisterInstance expects a non-nil interface type")
	}

	// Handle pointer to interface type (e.g., (*TestInterface)(nil))
	if ifaceType.Kind() == reflect.Ptr && ifaceType.Elem().Kind() == reflect.Interface {
		ifaceType = ifaceType.Elem()
	}

	if ifaceType.Kind() != reflect.Interface {
		panic("RegisterInstance expects an interface type or pointer to interface type")
	}

	if !reflect.TypeOf(instance).Implements(ifaceType) {
		panic(fmt.Sprintf("Instance does not implement interface %v", ifaceType))
	}

	c.services[ifaceType] = service{
		instance:      instance,
		isSingleton:   true,
		isInitialized: true,
	}

	return c
}

// RegisterSingleton registers a singleton service with the container.
// The service factory will be called only once, and the same instance is returned on every resolution.
func (c *Container) RegisterSingleton(iface any, factory func(*Container) (any, error)) *Container {
	return c.Register(iface, factory, true)
}

// RegisterTransient registers a transient service with the container.
// A new instance is created every time the service is resolved.
func (c *Container) RegisterTransient(iface any, factory func(*Container) (any, error)) *Container {
	return c.Register(iface, factory, false)
}

// Resolve resolves a service from the container.
// It returns the service instance or an error if the service is not registered or initialization fails.
// For singletons, the instance is created only once and cached for subsequent calls.
func (c *Container) Resolve(iface any) (any, error) {
	c.lock.RLock()
	ifaceType := reflect.TypeOf(iface)
	if ifaceType == nil {
		return nil, fmt.Errorf("Resolve expects a non-nil interface type")
	}

	// Handle pointer to interface type (e.g., (*TestInterface)(nil))
	if ifaceType.Kind() == reflect.Ptr && ifaceType.Elem().Kind() == reflect.Interface {
		ifaceType = ifaceType.Elem()
	}

	if ifaceType.Kind() != reflect.Interface {
		return nil, fmt.Errorf("Resolve expects an interface type or pointer to interface type")
	}

	svc, ok := c.services[ifaceType]
	c.lock.RUnlock()

	if !ok {
		return nil, fmt.Errorf("service not registered: %v", ifaceType)
	}

	// If it's a singleton and already initialized, return the instance
	if svc.isSingleton && svc.isInitialized {
		return svc.instance, nil
	}

	// For singletons, initialize it once
	if svc.isSingleton {
		c.lock.Lock()
		defer c.lock.Unlock()

		// Double-check if it was initialized while we were waiting for the lock
		updatedSvc, ok := c.services[ifaceType]
		if !ok {
			return nil, fmt.Errorf("service not registered: %v", ifaceType)
		}

		if updatedSvc.isInitialized {
			return updatedSvc.instance, nil
		}

		// Initialize the service
		instance, err := updatedSvc.factory(c)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize service: %w", err)
		}

		// Update the service in the map
		c.services[ifaceType] = service{
			instance:      instance,
			factory:       updatedSvc.factory,
			isSingleton:   updatedSvc.isSingleton,
			isInitialized: true,
		}

		return instance, nil
	}

	// For transient services, create a new instance every time
	instance, err := svc.factory(c)
	if err != nil {
		return nil, fmt.Errorf("failed to create service: %w", err)
	}

	return instance, nil
}

// ResolveTyped resolves a service from the container with type safety
func ResolveTyped[T any](c *Container) (T, error) {
	var zero T
	instance, err := c.Resolve((*T)(nil))
	if err != nil {
		return zero, err
	}
	typedInstance, ok := instance.(T)
	if !ok {
		return zero, fmt.Errorf("resolved instance is not of type %T", zero)
	}
	return typedInstance, nil
}

// MustResolveTyped resolves a service from the container with type safety and panics if it fails
func MustResolveTyped[T any](c *Container) T {
	instance, err := ResolveTyped[T](c)
	if err != nil {
		panic(err)
	}
	return instance
}

// MustResolve resolves a service from the container and panics if it fails
func (c *Container) MustResolve(iface any) any {
	instance, err := c.Resolve(iface)
	if err != nil {
		panic(err)
	}
	return instance
}

// Clear clears all registered services
func (c *Container) Clear() *Container {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.services = make(map[reflect.Type]service)
	return c
}

// Close closes all initialized singleton services that implement io.Closer.
// It returns a multi-error if any service fails to close.
func (c *Container) Close() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	var errs []error
	for _, svc := range c.services {
		if svc.isSingleton && svc.isInitialized && svc.instance != nil {
			if closer, ok := svc.instance.(interface{ Close() error }); ok {
				if err := closer.Close(); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to close some services: %v", errs)
	}
	return nil
}

// IsRegistered checks if a service is registered
func (c *Container) IsRegistered(iface any) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()

	ifaceType := reflect.TypeOf(iface)
	if ifaceType == nil {
		return false
	}

	// Handle pointer to interface type (e.g., (*TestInterface)(nil))
	if ifaceType.Kind() == reflect.Ptr && ifaceType.Elem().Kind() == reflect.Interface {
		ifaceType = ifaceType.Elem()
	}

	if ifaceType.Kind() != reflect.Interface {
		return false
	}

	_, ok := c.services[ifaceType]
	return ok
}

// IsRegisteredTyped checks if a service is registered with type safety
func IsRegisteredTyped[T any](c *Container) bool {
	return c.IsRegistered((*T)(nil))
}

// RegisterSingletonInstance registers a singleton instance
func RegisterSingletonInstance[T any](c *Container, instance T) *Container {
	return c.RegisterInstance((*T)(nil), instance)
}

// RegisterServices registers multiple services at once
func (c *Container) RegisterServices(services ...func(*Container) *Container) *Container {
	for _, service := range services {
		service(c)
	}
	return c
}

// Default returns the global default container
func Default() *Container {
	once.Do(func() {
		defaultContainer = New()
	})
	return defaultContainer
}

// Register registers a service with the default container
func Register(iface any, factory func(*Container) (any, error), isSingleton bool) *Container {
	return Default().Register(iface, factory, isSingleton)
}

// RegisterInstance registers an instance with the default container
func RegisterInstance(iface any, instance any) *Container {
	return Default().RegisterInstance(iface, instance)
}

// RegisterSingleton registers a singleton service with the default container
func RegisterSingleton(iface any, factory func(*Container) (any, error)) *Container {
	return Default().RegisterSingleton(iface, factory)
}

// RegisterTransient registers a transient service with the default container
func RegisterTransient(iface any, factory func(*Container) (any, error)) *Container {
	return Default().RegisterTransient(iface, factory)
}

// RegisterDefaultSingletonInstance registers a singleton instance with the default container
func RegisterDefaultSingletonInstance[T any](instance T) *Container {
	return RegisterSingletonInstance(Default(), instance)
}

// Resolve resolves a service from the default container
func Resolve(iface any) (any, error) {
	return Default().Resolve(iface)
}

// ResolveDefaultTyped resolves a service from the default container with type safety
func ResolveDefaultTyped[T any]() (T, error) {
	return ResolveTyped[T](Default())
}

// MustResolve resolves a service from the default container and panics if it fails
func MustResolve(iface any) any {
	return Default().MustResolve(iface)
}

// MustResolveDefaultTyped resolves a service from the default container with type safety and panics if it fails
func MustResolveDefaultTyped[T any]() T {
	return MustResolveTyped[T](Default())
}

// IsRegistered checks if a service is registered with the default container
func IsRegistered(iface any) bool {
	return Default().IsRegistered(iface)
}

// IsDefaultRegisteredTyped checks if a service is registered with the default container with type safety
func IsDefaultRegisteredTyped[T any]() bool {
	return IsRegisteredTyped[T](Default())
}

// Clear clears all registered services from the default container
func Clear() *Container {
	return Default().Clear()
}

// RegisterDefaultServices registers multiple services at once with the default container
func RegisterDefaultServices(services ...func(*Container) *Container) *Container {
	return Default().RegisterServices(services...)
}

// ResetForTesting clears the global default container for testing.
// This must only be called from test code to ensure clean state between tests.
// Production code must never call this function.
func ResetForTesting() {
	if defaultContainer != nil {
		defaultContainer.Clear()
	}
}
