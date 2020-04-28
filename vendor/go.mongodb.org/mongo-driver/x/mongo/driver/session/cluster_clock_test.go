// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package session

import (
	"bytes"
	"testing"

	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

func TestClusterClock(t *testing.T) {
	var clusterTime1 = bsoncore.BuildDocument(nil, bsoncore.AppendDocumentElement(nil, "$clusterTime", bsoncore.BuildDocument(nil, bsoncore.AppendTimestampElement(nil, "clusterTime", 10, 5))))
	var clusterTime2 = bsoncore.BuildDocument(nil, bsoncore.AppendDocumentElement(nil, "$clusterTime", bsoncore.BuildDocument(nil, bsoncore.AppendTimestampElement(nil, "clusterTime", 5, 5))))
	var clusterTime3 = bsoncore.BuildDocument(nil, bsoncore.AppendDocumentElement(nil, "$clusterTime", bsoncore.BuildDocument(nil, bsoncore.AppendTimestampElement(nil, "clusterTime", 5, 0))))

	t.Run("ClusterTime", func(t *testing.T) {
		clock := ClusterClock{}
		clock.AdvanceClusterTime(clusterTime3)
		done := make(chan struct{})
		go func() {
			clock.AdvanceClusterTime(clusterTime1)
			done <- struct{}{}
		}()
		clock.AdvanceClusterTime(clusterTime2)

		<-done
		if !bytes.Equal(clock.GetClusterTime(), clusterTime1) {
			t.Errorf("Expected cluster time %v, received %v", clusterTime1, clock.GetClusterTime())
		}
	})
}
