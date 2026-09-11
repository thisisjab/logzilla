package collector

import "context"

type CollectorCallback func(cname, d string) error

// Collector is the interface that all collectors must satisfy.
type Collector interface {
	// Collect is a blocking call func that will send read logs to dest by calling the callback.
	// Errors only will be returned only if collector cannot collect anymore.
	Collect(ctx context.Context, cb CollectorCallback) error
}
