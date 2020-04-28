// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package benchmark

import "testing"

func BenchmarkSingleRunCommand(b *testing.B)          { WrapCase(SingleRunCommand)(b) }
func BenchmarkSingleFindOneByID(b *testing.B)         { WrapCase(SingleFindOneByID)(b) }
func BenchmarkSingleInsertSmallDocument(b *testing.B) { WrapCase(SingleInsertSmallDocument)(b) }
func BenchmarkSingleInsertLargeDocument(b *testing.B) { WrapCase(SingleInsertLargeDocument)(b) }
