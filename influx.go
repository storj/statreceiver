// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package statreceiver

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/zeebo/errs"
)

// InfluxDest is a MetricDest that sends data with the Influx TCP wire
// protocol.
type InfluxDest struct {
	url   string
	token string

	mu      sync.Mutex
	buf     bytes.Buffer
	stopped bool
}

// NewInfluxDest creates a InfluxDest with stats URL url. Because
// this function is called in a Lua pipeline domain-specific language, the DSL
// wants a Influx destination to be flushing every few seconds, so this
// constructor will start that process. Use Close to stop it.
func NewInfluxDest(writeURL string) *InfluxDest {
	parsed, err := url.Parse(writeURL)
	if err != nil {
		panic(err)
	}
	token := parsed.Query().Get("authorization")
	parsed.Query().Del("authorization")

	rv := &InfluxDest{
		url:   parsed.String(),
		token: token,
	}
	go rv.flush()
	return rv
}

// Metric implements MetricDest.
func (d *InfluxDest) Metric(application, instance string, key []byte, val float64, ts time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// TODO(jeff): actual parsing of the key is very tricky in the presence of influx's busted
	// escapes. If we could do that, we could more easily put the application tag in sorted order
	// but since it begins with a, we'll do the easy thing and insert it first.
	added := false
keyRange:
	for i, val := range key {
		switch {
		case val != ' ' && val != ',':
			continue
		case i == 0:
			break keyRange
		case key[i-1] == '\\':
			continue
		}

		var newKey []byte
		newKey = append(newKey, key[:i]...)
		newKey = append(newKey, ",application="...)
		newKey = appendTag(newKey, application)

		// Only add instanceID if it's a satellite or retrievability-checker.
		if strings.Contains(application, "satellite") || strings.Contains(application, "retrievability") {
			newKey = append(newKey, ",instance="...)
			newKey = appendTag(newKey, instance)
		}

		newKey = append(newKey, key[i:]...)
		key = newKey

		added = true
		break
	}
	if !added {
		log.Printf("influx metric dropped: %q", key)
		return nil
	}

	_, err := fmt.Fprintf(&d.buf, "%s=%v %d\n", key, val, ts.Truncate(time.Second).UnixNano())
	return err
}

// appendTag writes a tag key, value, or field key to the buffer.
func appendTag(buf []byte, tag string) []byte {
	if !strings.ContainsAny(tag, ",= ") {
		return append(buf, tag...)
	}

	for i := 0; i < len(tag); i++ {
		if tag[i] == ',' ||
			tag[i] == '=' ||
			tag[i] == ' ' {
			buf = append(buf, '\\')
		}
		buf = append(buf, tag[i])
	}

	return buf
}

// Close stops the flushing goroutine.
func (d *InfluxDest) Close() error {
	d.mu.Lock()
	d.stopped = true
	d.mu.Unlock()
	return nil
}

func (d *InfluxDest) flush() {
	for {
		time.Sleep(5 * time.Second)

		d.mu.Lock()
		if d.stopped {
			d.mu.Unlock()
			return
		}
		data := append([]byte{}, d.buf.Bytes()...)
		d.buf.Reset()
		d.mu.Unlock()

		if len(data) == 0 {
			continue
		}

		err := func() (err error) {
			req, err := http.NewRequest("POST", d.url, bytes.NewReader(data))
			if err != nil {
				return err
			}
			if d.token != "" {
				req.Header.Set("Authorization", "Token "+d.token)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			defer func() { err = errs.Combine(err, resp.Body.Close()) }()

			if resp.StatusCode != http.StatusNoContent {
				return errs.New("invalid status code: %d", resp.StatusCode)
			}
			return nil
		}()
		if err != nil {
			log.Printf("failed flushing: %v", err)
		}
	}
}
