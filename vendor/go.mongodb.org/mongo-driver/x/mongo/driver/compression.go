// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package driver

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"

	"github.com/golang/snappy"
	"github.com/klauspost/compress/zstd"
	"go.mongodb.org/mongo-driver/x/mongo/driver/wiremessage"
)

// CompressionOpts holds settings for how to compress a payload
type CompressionOpts struct {
	Compressor       wiremessage.CompressorID
	ZlibLevel        int
	ZstdLevel        int
	UncompressedSize int32
}

// CompressPayload takes a byte slice and compresses it according to the options passed
func CompressPayload(in []byte, opts CompressionOpts) ([]byte, error) {
	switch opts.Compressor {
	case wiremessage.CompressorNoOp:
		return in, nil
	case wiremessage.CompressorSnappy:
		return snappy.Encode(nil, in), nil
	case wiremessage.CompressorZLib:
		var b bytes.Buffer
		w, err := zlib.NewWriterLevel(&b, opts.ZlibLevel)
		if err != nil {
			return nil, err
		}
		_, err = w.Write(in)
		if err != nil {
			return nil, err
		}
		err = w.Close()
		if err != nil {
			return nil, err
		}
		return b.Bytes(), nil
	case wiremessage.CompressorZstd:
		var b bytes.Buffer
		w, err := zstd.NewWriter(&b, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(opts.ZstdLevel)))
		if err != nil {
			return nil, err
		}
		_, err = io.Copy(w, bytes.NewBuffer(in))
		if err != nil {
			_ = w.Close()
			return nil, err
		}
		err = w.Close()
		if err != nil {
			return nil, err
		}
		return b.Bytes(), nil
	default:
		return nil, fmt.Errorf("unknown compressor ID %v", opts.Compressor)
	}
}

// DecompressPayload takes a byte slice that has been compressed and undoes it according to the options passed
func DecompressPayload(in []byte, opts CompressionOpts) ([]byte, error) {
	switch opts.Compressor {
	case wiremessage.CompressorNoOp:
		return in, nil
	case wiremessage.CompressorSnappy:
		uncompressed := make([]byte, opts.UncompressedSize)
		return snappy.Decode(uncompressed, in)
	case wiremessage.CompressorZLib:
		decompressor, err := zlib.NewReader(bytes.NewReader(in))
		if err != nil {
			return nil, err
		}
		uncompressed := make([]byte, opts.UncompressedSize)
		_, err = io.ReadFull(decompressor, uncompressed)
		if err != nil {
			return nil, err
		}
		return uncompressed, nil
	case wiremessage.CompressorZstd:
		w, err := zstd.NewReader(bytes.NewBuffer(in))
		if err != nil {
			return nil, err
		}
		defer w.Close()
		var b bytes.Buffer
		_, err = io.Copy(&b, w)
		if err != nil {
			return nil, err
		}
		return b.Bytes(), nil
	default:
		return nil, fmt.Errorf("unknown compressor ID %v", opts.Compressor)
	}
}
