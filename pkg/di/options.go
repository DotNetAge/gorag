package di

// Option represents a container option
type Option func(*Container)

// WithSingleton registers a service as a singleton
func WithSingleton(iface interface{}, factory func(*Container) (interface{}, error)) Option {
	return func(c *Container) {
		c.Register(iface, factory, true)
	}
}

// WithTransient registers a service as transient
func WithTransient(iface interface{}, factory func(*Container) (interface{}, error)) Option {
	return func(c *Container) {
		c.Register(iface, factory, false)
	}
}

// WithInstance registers an existing instance
func WithInstance(iface interface{}, instance interface{}) Option {
	return func(c *Container) {
		c.RegisterInstance(iface, instance)
	}
}

// NewWithOptions creates a new container with options
func NewWithOptions(opts ...Option) *Container {
	c := New()
	for _, opt := range opts {
		opt(c)
	}
	return c
}
