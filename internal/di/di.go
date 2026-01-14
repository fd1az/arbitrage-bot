package di

import "fmt"

// ServiceRegistry provides methods to register and retrieve services
type ServiceRegistry interface {
	// Register registers a service instance with the given name
	Register(name string, service interface{})

	// Get retrieves a service by name, returns interface{}
	Get(name string) interface{}

	// Has checks if a service is registered
	Has(name string) bool
}

// TypedServiceRegistry extends ServiceRegistry with type-safe retrieval
type TypedServiceRegistry[T any] interface {
	ServiceRegistry
	// GetTyped retrieves a service by name with type safety
	GetTyped(name string) T
}

// FactoryFunc is a function that creates a service instance
// It receives the registry to resolve dependencies
type FactoryFunc func(registry ServiceRegistry) interface{}

// Container extends ServiceRegistry with factory registration and building capabilities
type Container interface {
	ServiceRegistry

	// RegisterFactory registers a factory function to create a service
	RegisterFactory(name string, factory FactoryFunc)

	// Build constructs all registered services that haven't been built yet
	Build()
}

// Typed creates a typed adapter for a ServiceRegistry
func Typed[T any](registry ServiceRegistry) TypedServiceRegistry[T] {
	return &typedAdapter[T]{registry: registry}
}

// typedAdapter implements TypedServiceRegistry[T]
type typedAdapter[T any] struct {
	registry ServiceRegistry
}

// Register delegates to the underlying registry
func (t *typedAdapter[T]) Register(name string, service interface{}) {
	t.registry.Register(name, service)
}

// Get delegates to the underlying registry
func (t *typedAdapter[T]) Get(name string) interface{} {
	return t.registry.Get(name)
}

// Has delegates to the underlying registry
func (t *typedAdapter[T]) Has(name string) bool {
	return t.registry.Has(name)
}

// GetTyped retrieves a service with type safety
func (t *typedAdapter[T]) GetTyped(name string) T {
	service := t.registry.Get(name)
	typed, ok := service.(T)
	if !ok {
		var zero T
		panic(fmt.Sprintf("service '%s' is not of expected type %T", name, zero))
	}
	return typed
}
