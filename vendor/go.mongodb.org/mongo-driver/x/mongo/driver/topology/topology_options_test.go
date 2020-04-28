// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package topology

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
)

func TestOptionsSetting(t *testing.T) {
	conf := &config{}
	ssts := time.Minute
	name := "test"
	assert.Zero(t, conf.cs)

	opt := WithConnString(func(connstring.ConnString) connstring.ConnString {
		return connstring.ConnString{
			ServerSelectionTimeout:    ssts,
			ServerSelectionTimeoutSet: true,
			AppName:                   name,
		}

	})

	assert.NoError(t, opt(conf))

	assert.Equal(t, ssts, conf.serverSelectionTimeout)

	serverConf, err := newServerConfig(conf.serverOpts...)
	assert.Nil(t, err, "error from newServerConfig: %v", err)
	assert.Equal(t, name, serverConf.appname, "expected appname to be: %v, got: %v", name, serverConf.appname)
}
