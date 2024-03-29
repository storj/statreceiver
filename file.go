// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package statreceiver

import (
	"bufio"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// FileSource reads packets from a file.
type FileSource struct {
	path string

	mu      sync.Mutex
	decoder *gob.Decoder
}

// NewFileSource creates a FileSource.
func NewFileSource(path string) *FileSource {
	return &FileSource{path: path}
}

var _ Source = (*FileSource)(nil)

// Next implements the Source interface.
func (f *FileSource) Next() ([]byte, time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.decoder == nil {
		file, err := os.Open(f.path)
		if err != nil {
			return nil, time.Time{}, err
		}
		f.decoder = gob.NewDecoder(bufio.NewReader(file))
	}

	var p Packet
	err := f.decoder.Decode(&p)
	if err != nil {
		return nil, time.Time{}, err
	}
	return p.Data, p.TS, nil
}

// FileDest sends packets to a file for later processing. FileDest preserves
// the timestamps.
type FileDest struct {
	path string

	mu      sync.Mutex
	file    io.WriteCloser
	encoder *gob.Encoder
}

// NewFileDest creates a FileDest.
func NewFileDest(path string) *FileDest {
	return &FileDest{path: path}
}

var _ PacketDest = (*FileDest)(nil)

// Packet implements PacketDest.
func (f *FileDest) Packet(data []byte, ts time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.encoder == nil {
		file, err := os.Create(f.path)
		if err != nil {
			return err
		}
		f.file = file
		f.encoder = gob.NewEncoder(bufio.NewWriter(file))
	}

	return f.encoder.Encode(Packet{Data: data, TS: ts})
}

// Metric implements MetricDest.
func (f *FileDest) Metric(application, instance string, key []byte, val float64, ts time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.file == nil {
		file, err := os.Create(f.path)
		if err != nil {
			return err
		}
		f.file = file
	}

	_, err := fmt.Fprintf(f.file, "%s %s %s %v\n", application, instance, string(key), val)
	return err
}
