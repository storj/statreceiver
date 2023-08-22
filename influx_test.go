// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package statreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewInfluxDest_URLRedacted(t *testing.T) {
	t.Run("with password", func(t *testing.T) {
		const writeURL = "http://influx-host.test/write?u=username&p=pass"
		influx := NewInfluxDest(writeURL)
		assert.NotContains(t, influx.urlRedacted, "p=pass")
		assert.Contains(t, influx.urlRedacted, "p=REDACTED")
	})

	t.Run("without password", func(t *testing.T) {
		const writeURL = "http://influx-host.test/write?u=username"
		influx := NewInfluxDest(writeURL)
		assert.Equal(t, writeURL, influx.urlRedacted)
	})
}
