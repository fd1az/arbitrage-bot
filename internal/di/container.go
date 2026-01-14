package di

import (
	"fmt"
	"sync"
)

// container is the main implementation of Container interface
type container struct {
	services  map[string]interface{}     // Built services
	factories map[string]FactoryFunc     // Factory functions
	building  map[string]*sync.WaitGroup // Track services being built (circular dependency detection)
	mu        sync.RWMutex               // Thread safety
}

// NewContainer creates a new DI container
func NewContainer() Container {
	return &container{
		services:  make(map[string]interface{}),
		factories: make(map[string]FactoryFunc),
		building:  make(map[string]*sync.WaitGroup),
	}
}

// Register registers a service instance directly
func (c *container) Register(name string, service interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.services[name] = service
}

// RegisterFactory registers a factory function to create a service
func (c *container) RegisterFactory(name string, factory FactoryFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.factories[name] = factory
}

// Get retrieves a service by name
func (c *container) Get(name string) interface{} {
	c.mu.RLock()
	// Check if service is already built
	if service, exists := c.services[name]; exists {
		c.mu.RUnlock()
		return service
	}
	c.mu.RUnlock()

	// Check if factory exists
	c.mu.RLock()
	factory, exists := c.factories[name]
	c.mu.RUnlock()
	if !exists {
		panic(fmt.Sprintf("service '%s' not registered", name))
	}

	// Build the service
	return c.buildService(name, factory)
}

// Has checks if a service or factory is registered
func (c *container) Has(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, existsService := c.services[name]
	_, existsFactory := c.factories[name]
	return existsService || existsFactory
}

// buildService builds a service using its factory function
func (c *container) buildService(name string, factory FactoryFunc) interface{} {
	return c.buildServiceWithStack(name, factory, make(map[string]bool))
}

// buildServiceWithStack builds a service with circular dependency detection using call stack
func (c *container) buildServiceWithStack(name string, factory FactoryFunc, buildStack map[string]bool) interface{} {
	// Check for circular dependencies BEFORE acquiring any locks
	if buildStack[name] {
		panic(fmt.Sprintf("circular dependency detected for service '%s'", name))
	}

	c.mu.Lock()

	// Check if service was built while waiting for lock
	if service, exists := c.services[name]; exists {
		c.mu.Unlock()
		return service
	}

	// Check if another goroutine is building this service
	if wg, exists := c.building[name]; exists {
		c.mu.Unlock()
		// Wait for the other goroutine to finish building
		wg.Wait()
		// Now get the built service
		c.mu.RLock()
		service := c.services[name]
		c.mu.RUnlock()
		return service
	}

	// Mark as building by this goroutine
	wg := &sync.WaitGroup{}
	wg.Add(1)
	c.building[name] = wg

	// Unlock while building to allow dependency resolution
	c.mu.Unlock()

	// Create a copy of build stack to avoid race conditions
	newBuildStack := make(map[string]bool)
	for k, v := range buildStack {
		newBuildStack[k] = v
	}
	newBuildStack[name] = true

	// Create wrapper registry that passes the build stack
	wrapperRegistry := &buildStackRegistry{
		container:  c,
		buildStack: newBuildStack,
	}

	// Build the service
	service := factory(wrapperRegistry)

	// Lock again to store the result
	c.mu.Lock()
	c.services[name] = service
	delete(c.building, name)
	c.mu.Unlock()

	// Signal that building is complete
	wg.Done()

	return service
}

// buildStackRegistry wraps the container to pass build stack for circular dependency detection
type buildStackRegistry struct {
	container  *container
	buildStack map[string]bool
}

func (b *buildStackRegistry) Register(name string, service interface{}) {
	b.container.Register(name, service)
}

func (b *buildStackRegistry) Get(name string) interface{} {
	// Check if service is already built
	b.container.mu.RLock()
	if service, exists := b.container.services[name]; exists {
		b.container.mu.RUnlock()
		return service
	}

	// Check if factory exists
	factory, exists := b.container.factories[name]
	b.container.mu.RUnlock()

	if !exists {
		panic(fmt.Sprintf("service '%s' not registered", name))
	}

	// Build the service with current build stack
	return b.container.buildServiceWithStack(name, factory, b.buildStack)
}

func (b *buildStackRegistry) Has(name string) bool {
	return b.container.Has(name)
}

// Build constructs all registered services that haven't been built yet
func (c *container) Build() {
	c.mu.RLock()
	factories := make([]string, 0, len(c.factories))
	for name := range c.factories {
		if _, exists := c.services[name]; !exists {
			factories = append(factories, name)
		}
	}
	c.mu.RUnlock()

	// Build all pending services
	for _, name := range factories {
		c.Get(name)
	}
}
