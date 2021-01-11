// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package statreceiver

import (
	"log"
	"sync"
	"time"

	"github.com/zeebo/admission/v3/admproto"

	"storj.io/common/memory"
)

// Parser is a PacketDest that sends data to a MetricDest.
type Parser struct {
	dest    MetricDest
	scratch sync.Pool
}

// NewParser creates a Parser. It sends metrics to dest.
func NewParser(dest MetricDest) *Parser {
	return &Parser{
		dest: dest,
		scratch: sync.Pool{
			New: func() interface{} {
				var x [10 * memory.KB]byte
				return &x
			},
		},
	}
}

// Packet implements PacketDest.
func (p *Parser) Packet(data []byte, ts time.Time) (err error) {
	data, err = admproto.CheckChecksum(data)
	if err != nil {
		return err
	}

	scratch := p.scratch.Get().(*[10 * memory.KB]byte)
	defer p.scratch.Put(scratch)

	r := admproto.NewReaderWith((*scratch)[:])
	data, appb, instb, numHeaders, err := r.Begin(data)
	if err != nil {
		return err
	}

	// Even though we don't use the headers, if they exist on the buffer we
	// need to read them off.
	for i := 0; i < numHeaders; i++ {
		data, _, _, err = r.NextHeader(data)
		if err != nil {
			return err
		}
	}

	app, inst := string(appb), string(instb)
	var key []byte
	var value float64
	for len(data) > 0 {
		data, key, value, err = r.Next(data)
		if err != nil {
			return err
		}
		err = p.dest.Metric(app, inst, key, value, ts)
		if err != nil {
			log.Printf("failed to write metric: %v", err)
			continue
		}
	}

	return nil
}
