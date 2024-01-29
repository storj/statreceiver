// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package statreceiver

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"storj.io/eventkit"
)

var ek = eventkit.Package()

// Eventkit sends metrics to eventkit endpoint.
type Eventkit struct {
	mu     sync.Mutex
	name   string
	cancel func()
	addr   string
}

// NewEventkit creates a FileSource.
func NewEventkit(addr string) *Eventkit {
	return &Eventkit{
		addr: addr,
	}
}

// Metric implements MetricDest.
func (e *Eventkit) Metric(application, instance string, key []byte, val float64, ts time.Time) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cancel == nil {
		var ctx context.Context
		ctx, e.cancel = context.WithCancel(context.Background())
		hostname, _ := os.Hostname()
		client := eventkit.NewUDPClient("statreceiver", "", hostname, e.addr)
		eventkit.DefaultRegistry.AddDestination(client)
		go client.Run(ctx)
	}
	tags := []eventkit.Tag{
		eventkit.String("application", application),
		eventkit.String("instance", instance),
		eventkit.Float64("value", val),
		eventkit.Timestamp("ts", ts),
	}
	scopeField := strings.Split(string(key), " ")
	if len(scopeField) > 1 {
		tags = append(tags, eventkit.String("field", scopeField[1]))
	}
	taggedName := strings.Split(scopeField[0], ",")
	tags = append(tags, eventkit.String("name", taggedName[0]))
	for i := 1; i < len(taggedName)-1; i++ {
		kv := strings.SplitN(taggedName[i], "=", 2)
		tags = append(tags, eventkit.String(kv[0], kv[1]))
	}

	ek.Event(e.name, tags...)
	return nil
}

// Close implements io.Closer.
func (e *Eventkit) Close() error {
	e.cancel()
	return nil
}

var _ MetricDest = (*Eventkit)(nil)
