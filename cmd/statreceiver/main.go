// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/common/fpath"
	"storj.io/private/cfgstruct"
	"storj.io/private/process"
	"storj.io/statreceiver"
	"storj.io/statreceiver/luacfg"
)

// Config is the set of configuration values we care about.
var Config struct {
	Input string `default:"" help:"path to configuration file"`
}

func main() {
	defaultConfDir := fpath.ApplicationDir("storj", "statreceiver")

	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "stat receiving",
		RunE:  Main,
	}
	defaults := cfgstruct.DefaultsFlag(cmd)
	process.Bind(cmd, &Config, defaults, cfgstruct.ConfDir(defaultConfDir))
	cmd.Flags().String("config", filepath.Join(defaultConfDir, "config.yaml"), "path to configuration")
	process.Exec(cmd)
}

// Main is the real main method.
func Main(cmd *cobra.Command, args []string) error {
	var input io.Reader
	switch Config.Input {
	case "":
		return fmt.Errorf("--input path to script is required")
	case "stdin":
		input = os.Stdin
	default:
		inputFile, err := os.Open(Config.Input)
		if err != nil {
			return err
		}
		defer func() {
			if err := inputFile.Close(); err != nil {
				log.Printf("Failed to close input: %scope", err)
			}
		}()

		input = inputFile
	}

	scope := luacfg.NewScope()
	err := errs.Combine(
		scope.RegisterVal("deliver", statreceiver.Deliver),
		scope.RegisterVal("filein", statreceiver.NewFileSource),
		scope.RegisterVal("fileout", statreceiver.NewFileDest),
		scope.RegisterVal("udpin", statreceiver.NewUDPSource),
		scope.RegisterVal("udpout", statreceiver.NewUDPDest),
		scope.RegisterVal("parse", statreceiver.NewParser),
		scope.RegisterVal("print", statreceiver.NewPrinter),
		scope.RegisterVal("packetprint", statreceiver.NewPacketPrinter),
		scope.RegisterVal("pcopy", statreceiver.NewPacketCopier),
		scope.RegisterVal("mcopy", statreceiver.NewMetricCopier),
		scope.RegisterVal("pbuf", statreceiver.NewPacketBuffer),
		scope.RegisterVal("mbuf", statreceiver.NewMetricBuffer),
		scope.RegisterVal("packetfilter", statreceiver.NewPacketFilter),
		scope.RegisterVal("headermultivalmatcher", statreceiver.NewHeaderMultiValMatcher),
		scope.RegisterVal("appfilter", statreceiver.NewApplicationFilter),
		scope.RegisterVal("instfilter", statreceiver.NewInstanceFilter),
		scope.RegisterVal("keyfilter", statreceiver.NewKeyFilter),
		scope.RegisterVal("sanitize", statreceiver.NewSanitizer),
		scope.RegisterVal("influxsanitizer", statreceiver.NewInfluxSanitizer),
		scope.RegisterVal("graphite", statreceiver.NewGraphiteDest),
		scope.RegisterVal("influx", statreceiver.NewInfluxDest),
		scope.RegisterVal("db", statreceiver.NewDBDest),
		scope.RegisterVal("pbufprep", statreceiver.NewPacketBufPrep),
		scope.RegisterVal("mbufprep", statreceiver.NewMetricBufPrep),
		scope.RegisterVal("versionsplit", statreceiver.NewVersionSplit),
	)
	if err != nil {
		return err
	}

	err = scope.Run(input)
	if err != nil {
		return err
	}

	log.Printf("Started")

	select {}
}
