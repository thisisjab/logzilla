package storage

import "context"

type Storage interface {
	Open(ctx context.Context) error
	Close(ctx context.Context) error
}
