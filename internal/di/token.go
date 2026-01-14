package di

import (
	"fmt"
	"reflect"
)

// Token represents a typed dependency injection token
// It provides type-safe access to services in the container
type Token[T any] struct {
	key         string
	serviceType reflect.Type
}

// NewToken creates a new typed token for a service
func NewToken[T any](key string) Token[T] {
	var zero T
	return Token[T]{
		key:         key,
		serviceType: reflect.TypeOf(&zero).Elem(),
	}
}

// Key returns the string key for this token
func (t Token[T]) Key() string {
	return t.key
}

// Type returns the reflect.Type of the service
func (t Token[T]) Type() reflect.Type {
	return t.serviceType
}

// String implements the Stringer interface
func (t Token[T]) String() string {
	return fmt.Sprintf("Token[%s](%s)", t.serviceType, t.key)
}

// RegisterToken registers a service factory with a typed token
func RegisterToken[T any](c Container, token Token[T], factory func(ServiceRegistry) T) {
	c.RegisterFactory(token.Key(), func(sr ServiceRegistry) interface{} {
		return factory(sr)
	})
}

// GetToken retrieves a service using a typed token
func GetToken[T any](c ServiceRegistry, token Token[T]) T {
	service := c.Get(token.Key())
	if service == nil {
		panic(fmt.Sprintf("service for token %s not found", token))
	}

	typed, ok := service.(T)
	if !ok {
		panic(fmt.Sprintf("service for token %s is not of expected type %T, got %T",
			token, typed, service))
	}

	return typed
}

// MustGetToken retrieves a service using a typed token, panics if not found or wrong type
func MustGetToken[T any](c ServiceRegistry, token Token[T]) T {
	return GetToken(c, token)
}

// HasToken checks if a service is registered for the given token
func HasToken[T any](c ServiceRegistry, token Token[T]) bool {
	return c.Has(token.Key())
}

// TypedContainer wraps a Container with type-safe methods
type TypedContainer struct {
	Container
}

// NewTypedContainer creates a new TypedContainer wrapping the given container
func NewTypedContainer(c Container) *TypedContainer {
	return &TypedContainer{Container: c}
}
