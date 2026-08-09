package collector

import "context"

// Collector is the interface that all collectors must satisfy.
type Collector interface {
	// Collect is a blocking call func that will send read logs to dest.
	// Errors only will be returned only if collector cannot collect anymore.
	Collect(ctx context.Context, dest chan<- string) error
}
