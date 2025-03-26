package graph

import "context"

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct{}

func (r *queryResolver) Hello(ctx context.Context) (string, error) {
	return "Hello, world!", nil
}
