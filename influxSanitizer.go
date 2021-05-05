// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package statreceiver

import (
	"regexp"
	"time"
)

// sanitizePattern matches the following pattern:
// 1 or more nonalphanumeric except space.
var sanitizePattern = regexp.MustCompile(`([^a-zA-Z\\d\\s]+)`)

// InfluxSanitizer is a MetricDest that replaces nonalphanumeric (except spaces) chars by
// underscores.
type InfluxSanitizer struct {
	dest MetricDest
}

// NewInfluxSanitizer creates a Sanitizer that sends sanitized metrics to dest.
func NewInfluxSanitizer(dest MetricDest) *InfluxSanitizer { return &InfluxSanitizer{dest: dest} }

// Metric implements MetricDest.
func (s *InfluxSanitizer) Metric(application, instance string, key []byte, val float64, ts time.Time) error {
	return s.dest.Metric(influxSanitize(application), influxSanitize(instance), influxSanitizeb(key), val, ts)
}

func influxSanitize(val string) string {
	return sanitizePattern.ReplaceAllString(val, "_")
}

func influxSanitizeb(val []byte) []byte {
	return sanitizePattern.ReplaceAll(val, []byte("_"))
}
