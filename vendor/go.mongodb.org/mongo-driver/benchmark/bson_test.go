// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package benchmark

import "testing"

// func BenchmarkBSONFullReaderDecoding(b *testing.B)       { WrapCase(BSONFullReaderDecoding)(b) }

func BenchmarkBSONFlatDocumentEncoding(b *testing.B)     { WrapCase(BSONFlatDocumentEncoding)(b) }
func BenchmarkBSONFlatDocumentDecodingLazy(b *testing.B) { WrapCase(BSONFlatDocumentDecodingLazy)(b) }
func BenchmarkBSONFlatDocumentDecoding(b *testing.B)     { WrapCase(BSONFlatDocumentDecoding)(b) }
func BenchmarkBSONDeepDocumentEncoding(b *testing.B)     { WrapCase(BSONDeepDocumentEncoding)(b) }
func BenchmarkBSONDeepDocumentDecodingLazy(b *testing.B) { WrapCase(BSONDeepDocumentDecodingLazy)(b) }
func BenchmarkBSONDeepDocumentDecoding(b *testing.B)     { WrapCase(BSONDeepDocumentDecoding)(b) }

// func BenchmarkBSONFullDocumentEncoding(b *testing.B)     { WrapCase(BSONFullDocumentEncoding)(b) }
// func BenchmarkBSONFullDocumentDecodingLazy(b *testing.B) { WrapCase(BSONFullDocumentDecodingLazy)(b) }
// func BenchmarkBSONFullDocumentDecoding(b *testing.B)     { WrapCase(BSONFullDocumentDecoding)(b) }

func BenchmarkBSONFlatMapDecoding(b *testing.B) { WrapCase(BSONFlatMapDecoding)(b) }
func BenchmarkBSONFlatMapEncoding(b *testing.B) { WrapCase(BSONFlatMapEncoding)(b) }
func BenchmarkBSONDeepMapDecoding(b *testing.B) { WrapCase(BSONDeepMapDecoding)(b) }
func BenchmarkBSONDeepMapEncoding(b *testing.B) { WrapCase(BSONDeepMapEncoding)(b) }

// func BenchmarkBSONFullMapDecoding(b *testing.B)       { WrapCase(BSONFullMapDecoding)(b) }
// func BenchmarkBSONFullMapEncoding(b *testing.B)       { WrapCase(BSONFullMapEncoding)(b) }

func BenchmarkBSONFlatStructDecoding(b *testing.B)     { WrapCase(BSONFlatStructDecoding)(b) }
func BenchmarkBSONFlatStructTagsDecoding(b *testing.B) { WrapCase(BSONFlatStructTagsDecoding)(b) }
func BenchmarkBSONFlatStructEncoding(b *testing.B)     { WrapCase(BSONFlatStructEncoding)(b) }
func BenchmarkBSONFlatStructTagsEncoding(b *testing.B) { WrapCase(BSONFlatStructTagsEncoding)(b) }
