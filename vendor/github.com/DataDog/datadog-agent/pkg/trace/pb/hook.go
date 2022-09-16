// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package pb

import (
	"sync"
)

var (
	mu       sync.RWMutex // guards metahook
	metahook func(_, v string) string
)

// SetMetaHook registers a callback which will run upon decoding each map
// entry in the span's Meta field. The hook has the opportunity to alter the
// value that is assigned to span.Meta[k] at decode time. By default, if no
// hook is defined, the behaviour is span.Meta[k] = v.
func SetMetaHook(hook func(k, v string) string) {
	mu.Lock()
	defer mu.Unlock()
	metahook = hook
}

// MetaHook returns the active meta hook. A MetaHook is a function which is ran
// for each span.Meta[k] = v value and has the opportunity to alter the final v.
func MetaHook() (hook func(k, v string) string, ok bool) {
	mu.RLock()
	defer mu.RUnlock()
	return metahook, metahook != nil
}
