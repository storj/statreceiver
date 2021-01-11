// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package statreceiver

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// GraphiteDest is a MetricDest that sends data with the Graphite TCP wire
// protocol.
type GraphiteDest struct {
	address string

	mu      sync.Mutex
	conn    net.Conn
	buf     *bufio.Writer
	stopped bool
}

// NewGraphiteDest creates a GraphiteDest with TCP address address. Because
// this function is called in a Lua pipeline domain-specific language, the DSL
// wants a graphite destination to be flushing every few seconds, so this
// constructor will start that process. Use Close to stop it.
func NewGraphiteDest(address string) *GraphiteDest {
	rv := &GraphiteDest{address: address}
	go rv.run()
	return rv
}

// Metric implements MetricDest.
func (d *GraphiteDest) Metric(application, instance string, key []byte, val float64, ts time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.stopped {
		return errors.New("graphite dest is stopped, cannot add metric")
	}

	if d.conn == nil {
		err := d.establishConnection()
		if err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(d.buf, "%s.%s.%s %v %d\n", application, instance, string(key), val, ts.Unix())
	if err != nil {
		log.Printf("err writing to buffer, re-establishing connection. err: %v", err)
		err = d.establishConnection()
		if err != nil {
			return err
		}
		// Try one more time, now that we've reestablished connection.
		_, err = fmt.Fprintf(d.buf, "%s.%s.%s %v %d\n", application, instance, string(key), val, ts.Unix())
	}
	return err
}

// establishConnection dials the address and prepares a buffer for writing.
// Only call while holding the mutex lock.
func (d *GraphiteDest) establishConnection() error {
	if d.conn != nil {
		err := d.conn.Close()
		if err != nil {
			log.Printf("failed closing old conn: %v", err)
		}
	}
	conn, err := net.Dial("tcp", d.address)
	if err != nil {
		d.conn = nil
		return err
	}
	d.conn = conn
	d.buf = bufio.NewWriter(conn)
	return nil
}

// Close stops the flushing goroutine.
func (d *GraphiteDest) Close() (err error) {
	d.mu.Lock()
	d.stopped = true
	if d.conn != nil {
		err = d.conn.Close()
	}
	d.mu.Unlock()
	return err
}

// run periodically flushes the buffer to the underlying conn.
func (d *GraphiteDest) run() {
	for {
		time.Sleep(5 * time.Second)
		d.mu.Lock()
		if d.stopped {
			d.mu.Unlock()
			return
		}
		var err error
		if d.buf != nil {
			err = d.buf.Flush()
		}
		d.mu.Unlock()
		if err != nil {
			log.Printf("failed flushing: %v", err)
		}
	}
}

// Flush manually flushes the buffer to the underlying writer.
func (d *GraphiteDest) Flush() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.buf.Flush()
}

// TestCloseConn is used for tests only, and allows you to manually close the
// underlying connection to trigger failure states.
func (d *GraphiteDest) TestCloseConn() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.conn.Close()
}
