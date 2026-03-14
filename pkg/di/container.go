package di

import (
	"fmt"
	"reflect"
	"sync"
)

// Container represents a dependency injection container
type Container struct {
	services map[reflect.Type]service
	lock     sync.RWMutex
}

// service represents a registered service
type service struct {
	instance     interface{}
	factory      func(*Container) (interface{}, error)
	isSingleton  bool
	isInitialized bool
}

// New creates a new dependency injection container
func New() *Container {
	return &Container{
		services: make(map[reflect.Type]service),
	}
}

// Register registers a service with the container
func (c *Container) Register(iface interface{}, factory func(*Container) (interface{}, error), isSingleton bool) {
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
}

// RegisterInstance registers an existing instance with the container
func (c *Container) RegisterInstance(iface interface{}, instance interface{}) {
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
		instance:     instance,
		isSingleton:  true,
		isInitialized: true,
	}
}

// Resolve resolves a service from the container
func (c *Container) Resolve(iface interface{}) (interface{}, error) {
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
			instance:     instance,
			factory:      updatedSvc.factory,
			isSingleton:  updatedSvc.isSingleton,
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

// MustResolve resolves a service from the container and panics if it fails
func (c *Container) MustResolve(iface interface{}) interface{} {
	instance, err := c.Resolve(iface)
	if err != nil {
		panic(err)
	}
	return instance
}

// Clear clears all registered services
func (c *Container) Clear() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.services = make(map[reflect.Type]service)
}

// IsRegistered checks if a service is registered
func (c *Container) IsRegistered(iface interface{}) bool {
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
