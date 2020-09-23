package storage

import (
	"context"
	"time"

	"github.com/safing/portbase/database/iterator"
	"github.com/safing/portbase/database/query"
	"github.com/safing/portbase/database/record"
)

// Interface defines the database storage API.
type Interface interface {
	Get(key string) (record.Record, error)
	Put(m record.Record) (record.Record, error)
	Delete(key string) error
	Query(q *query.Query, local, internal bool) (*iterator.Iterator, error)

	ReadOnly() bool
	Injected() bool
	Shutdown() error
}

// Maintenance defines the database storage API for backends that requ
type Maintenance interface {
	Maintain(ctx context.Context) error
	MaintainThorough(ctx context.Context) error
	MaintainRecordStates(ctx context.Context, purgeDeletedBefore time.Time) error
}

// Batcher defines the database storage API for backends that support batch operations.
type Batcher interface {
	PutMany(shadowDelete bool) (batch chan<- record.Record, errs <-chan error)
}
