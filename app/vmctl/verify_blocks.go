package main

import (
	"flag"
	"log"
	"os"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/native"
)

var (
	gunzip    = flag.Bool("gunzip", false, "Use GNU zip decompression for exported block")
	blockPath = flag.String("blockPath", "", "Defines path to verified blocks")
)

func verifyBlocks(args []string) {
	if len(*blockPath) == 0 {
		logger.Fatalf("you must provide path for exported data block")
	}
	common.StartUnmarshalWorkers()
	path := *blockPath
	isBlockGzipped := *gunzip
	log.Printf("verifying block at path=%q", path)
	f, err := os.OpenFile(path, os.O_RDONLY, 0600)
	if err != nil {
		logger.Fatalf("cannot open exported block at path=%q err=%s", path, err)
	}
	var blocksCount uint64
	if err := parser.ParseStream(f, isBlockGzipped, func(block *parser.Block) error {
		atomic.AddUint64(&blocksCount, 1)
		return nil
	}); err != nil {
		logger.Fatalf("cannot parse block at path=%q, blocksCount=%d, err=%s", path, blocksCount, err)
	}
	log.Printf("successfully verified block at path=%q, blockCount=%d", path, blocksCount)
}
