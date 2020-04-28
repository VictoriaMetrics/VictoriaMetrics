// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bsonx

import (
	"fmt"
)

func ExampleArray() {
	internalVersion := "1234567"

	f := func(appName string) Arr {
		arr := make(Arr, 0)
		arr = append(arr,
			Document(Doc{{"name", String("mongo-go-driver")}, {"version", String(internalVersion)}}),
			Document(Doc{{"type", String("darwin")}, {"architecture", String("amd64")}}),
			String("go1.9.2"),
		)
		if appName != "" {
			arr = append(arr, Document(MDoc{"name": String(appName)}))
		}

		return arr
	}
	_, buf, err := f("hello-world").MarshalBSONValue()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(buf)

	// Output: [154 0 0 0 3 48 0 52 0 0 0 2 110 97 109 101 0 16 0 0 0 109 111 110 103 111 45 103 111 45 100 114 105 118 101 114 0 2 118 101 114 115 105 111 110 0 8 0 0 0 49 50 51 52 53 54 55 0 0 3 49 0 46 0 0 0 2 116 121 112 101 0 7 0 0 0 100 97 114 119 105 110 0 2 97 114 99 104 105 116 101 99 116 117 114 101 0 6 0 0 0 97 109 100 54 52 0 0 2 50 0 8 0 0 0 103 111 49 46 57 46 50 0 3 51 0 27 0 0 0 2 110 97 109 101 0 12 0 0 0 104 101 108 108 111 45 119 111 114 108 100 0 0 0]
}
