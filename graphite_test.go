// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package statreceiver_test

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"storj.io/statreceiver"
)

func TestGraphiteDest(t *testing.T) {
	t.Skip("Not working on Jenkins, yet")
	l, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err)

	var eg errgroup.Group

	eg.Go(func() error {
		var conns []net.Conn
		for {
			conn, err := l.Accept()
			if err != nil {
				break
			}
			conns = append(conns, conn)
		}
		for _, conn := range conns {
			require.NoError(t, conn.Close())
		}
		return nil
	})

	address := l.Addr().String()

	gd := statreceiver.NewGraphiteDest(address)

	// first write a metric, which will establish the connection
	require.NoError(t, gd.Metric("bob", "jones", []byte("key"), 10.1, time.Now()))

	// Close the conn to trigger an error state, where we can no longer write to
	// the conn.
	require.NoError(t, gd.TestCloseConn())

	// Flush the metric to the underlying writer, this should cause an error due
	// to the closed conn.
	require.Error(t, gd.Flush())

	// We should still be able to successfully write a metric, and should
	// establish a new conn. Flush should also work.
	require.NoError(t, gd.Metric("bob", "jones", []byte("key"), 10.1, time.Now()))
	require.NoError(t, gd.Flush())
	require.NoError(t, gd.Close())
	require.NoError(t, l.Close())
	require.NoError(t, eg.Wait())
}
