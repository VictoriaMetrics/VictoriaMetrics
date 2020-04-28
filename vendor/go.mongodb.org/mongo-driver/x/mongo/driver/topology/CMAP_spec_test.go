package topology

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/event"
	testHelpers "go.mongodb.org/mongo-driver/internal/testutil/helpers"
	"go.mongodb.org/mongo-driver/x/mongo/driver/address"
)

type cmapEvent struct {
	EventType    string      `json:"type"`
	Address      interface{} `json:"address"`
	ConnectionID uint64      `json:"connectionId"`
	Options      interface{} `json:"options"`
	Reason       string      `json:"reason"`
}

type poolOptions struct {
	MaxPoolSize        int32 `json:"maxPoolSize"`
	MinPoolSize        int32 `json:"minPoolSize"`
	MaxIdleTimeMS      int32 `json:"maxIdleTimeMS"`
	WaitQueueTimeoutMS int32 `json:"waitQueueTimeoutMS"`
}

type cmapTestFile struct {
	Version     uint64                   `json:"version"`
	Style       string                   `json:"style"`
	Description string                   `json:"description"`
	SkipReason  string                   `json:"skipReason"`
	PoolOptions poolOptions              `json:"poolOptions"`
	Operations  []map[string]interface{} `json:"operations"`
	Error       *cmapTestError           `json:"error"`
	Events      []cmapEvent              `json:"events"`
	Ignore      []string                 `json:"ignore"`
}

type cmapTestError struct {
	ErrorType string `json:"type"`
	Message   string `json:"message"`
	Address   string `json:"address"`
}

type simThread struct {
	JobQueue      chan func()
	JobsAssigned  int32
	JobsCompleted int32
}

type testInfo struct {
	objects                map[string]interface{}
	originalEventChan      chan *event.PoolEvent
	finalEventChan         chan *event.PoolEvent
	threads                map[string]*simThread
	backgroundThreadErrors chan error
	eventCounts            map[string]uint64
	sync.Mutex
}

const cmapTestDir = "../../../../data/connection-monitoring-and-pooling/"

func TestCMAP(t *testing.T) {
	for _, testFileName := range testHelpers.FindJSONFilesInDir(t, cmapTestDir) {
		t.Run(testFileName, func(t *testing.T) {
			runCMAPTest(t, testFileName)
		})
	}
}

func runCMAPTest(t *testing.T, testFileName string) {
	content, err := ioutil.ReadFile(path.Join(cmapTestDir, testFileName))
	testHelpers.RequireNil(t, err, "unable to read content of test file")

	var test cmapTestFile
	err = json.Unmarshal(content, &test)
	testHelpers.RequireNil(t, err, "error unmarshalling testFile %v", err)

	if test.SkipReason != "" {
		t.Skip(test.SkipReason)
	}

	testInfo := &testInfo{
		objects:                make(map[string]interface{}),
		originalEventChan:      make(chan *event.PoolEvent, 200),
		finalEventChan:         make(chan *event.PoolEvent, 200),
		threads:                make(map[string]*simThread),
		eventCounts:            make(map[string]uint64),
		backgroundThreadErrors: make(chan error, 100),
	}

	l, err := net.Listen("tcp", "localhost:0")
	testHelpers.RequireNil(t, err, "unable to create listener: %v", err)

	s, err := NewServer(address.Address(l.Addr().String()),
		WithMaxConnections(func(u uint64) uint64 {
			return uint64(test.PoolOptions.MaxPoolSize)
		}),
		WithMinConnections(func(u uint64) uint64 {
			return uint64(test.PoolOptions.MinPoolSize)
		}),
		WithConnectionPoolMaxIdleTime(func(duration time.Duration) time.Duration {
			return time.Duration(test.PoolOptions.MaxIdleTimeMS) * time.Millisecond
		}),
		WithConnectionPoolMonitor(func(monitor *event.PoolMonitor) *event.PoolMonitor {
			return &event.PoolMonitor{func(event *event.PoolEvent) { testInfo.originalEventChan <- event }}
		}))
	testHelpers.RequireNil(t, err, "error creating server: %v", err)
	s.connectionstate = connected
	err = s.pool.connect()
	testHelpers.RequireNil(t, err, "error connecting connection pool: %v", err)

	for _, op := range test.Operations {
		if tempErr := runOperation(t, op, testInfo, s, test.PoolOptions.WaitQueueTimeoutMS); tempErr != nil {
			if err != nil {
				t.Fatalf("recieved multiple errors in primary thread: %v and %v", err, tempErr)
			}
			err = tempErr
		}
	}

	// make sure all threads have finished
	testInfo.Lock()
	threadNames := make([]string, 0)
	for threadName := range testInfo.threads {
		threadNames = append(threadNames, threadName)
	}
	testInfo.Unlock()

	for _, threadName := range threadNames {
	WAIT:
		for {
			testInfo.Lock()
			thread, ok := testInfo.threads[threadName]
			if !ok {
				t.Fatalf("thread was unexpectedly ended: %v", threadName)
			}
			if len(thread.JobQueue) == 0 && atomic.LoadInt32(&thread.JobsCompleted) == atomic.LoadInt32(&thread.JobsAssigned) {
				break WAIT
			}
			testInfo.Unlock()
		}
		close(testInfo.threads[threadName].JobQueue)
		testInfo.Unlock()
	}

	if test.Error != nil {
		if err == nil || strings.ToLower(test.Error.Message) != err.Error() {
			var erroredCorrectly bool
			errs := make([]error, 0, len(testInfo.backgroundThreadErrors)+1)
			errs = append(errs, err)
			for len(testInfo.backgroundThreadErrors) > 0 {
				bgErr := <-testInfo.backgroundThreadErrors
				errs = append(errs, bgErr)
				if bgErr != nil && strings.ToLower(test.Error.Message) == bgErr.Error() {
					erroredCorrectly = true
					break
				}
			}
			if !erroredCorrectly {
				t.Fatalf("error differed from expected error, expected: %v, actual errors recieved: %v", test.Error.Message, errs)
			}
		}
	}

	testInfo.Lock()
	defer testInfo.Unlock()
	for len(testInfo.originalEventChan) > 0 {
		temp := <-testInfo.originalEventChan
		testInfo.finalEventChan <- temp
	}

	checkEvents(t, test.Events, testInfo.finalEventChan, test.Ignore)

}

func checkEvents(t *testing.T, expectedEvents []cmapEvent, actualEvents chan *event.PoolEvent, ignoreEvents []string) {
	for _, expectedEvent := range expectedEvents {
		validEvent := nextValidEvent(t, actualEvents, ignoreEvents)

		if expectedEvent.EventType != validEvent.Type {
			var reason string
			if validEvent.Type == "ConnectionCheckOutFailed" {
				reason = ": " + validEvent.Reason
			}
			t.Fatalf("unexpected event occured: expected: %v, actual: %v%v", expectedEvent.EventType, validEvent.Type, reason)
		}

		if expectedEvent.Address != nil {

			if expectedEvent.Address == float64(42) { // can be any address
				if validEvent.Address == "" {
					t.Fatalf("expected address in event, instead recieved none in %v", expectedEvent.EventType)
				}
			} else { // must be specific address
				addr, ok := expectedEvent.Address.(string)
				if !ok {
					t.Fatalf("recieved non string address: %v", expectedEvent.Address)
				}
				if addr != validEvent.Address {
					t.Fatalf("recieved unexpected address: %v, expected: %v", validEvent.Address, expectedEvent.Address)
				}
			}
		}

		if expectedEvent.ConnectionID != 0 {
			if expectedEvent.ConnectionID == 42 {
				if validEvent.ConnectionID == 0 {
					t.Fatalf("expected a connectionId but found none in %v", validEvent.Type)
				}
			} else if expectedEvent.ConnectionID != validEvent.ConnectionID {
				t.Fatalf("expected and actual connectionIds differed: expected: %v, actual: %v for event: %v", expectedEvent.ConnectionID, validEvent.ConnectionID, expectedEvent.EventType)
			}
		}

		if expectedEvent.Reason != "" && expectedEvent.Reason != validEvent.Reason {
			t.Fatalf("event reason differed from expected: expected: %v, actual: %v for %v", expectedEvent.Reason, validEvent.Reason, expectedEvent.EventType)
		}

		if expectedEvent.Options != nil {
			if expectedEvent.Options == float64(42) {
				if validEvent.PoolOptions == nil {
					t.Fatalf("expected poolevent options but found none")
				}
			} else {
				opts, ok := expectedEvent.Options.(map[string]interface{})
				if !ok {
					t.Fatalf("event options were unexpected type: %T for %v", expectedEvent.Options, expectedEvent.EventType)
				}

				if maxSize, ok := opts["maxPoolSize"]; ok && validEvent.PoolOptions.MaxPoolSize != uint64(maxSize.(float64)) {
					t.Fatalf("event's max pool size differed from expected: %v, actual: %v", maxSize, validEvent.PoolOptions.MaxPoolSize)
				}

				if minSize, ok := opts["minPoolSize"]; ok && validEvent.PoolOptions.MinPoolSize != uint64(minSize.(float64)) {
					t.Fatalf("event's min pool size differed from expected: %v, actual: %v", minSize, validEvent.PoolOptions.MinPoolSize)
				}

				if waitQueueTimeoutMS, ok := opts["waitQueueTimeoutMS"]; ok && validEvent.PoolOptions.WaitQueueTimeoutMS != uint64(waitQueueTimeoutMS.(float64)) {
					t.Fatalf("event's min pool size differed from expected: %v, actual: %v", waitQueueTimeoutMS, validEvent.PoolOptions.WaitQueueTimeoutMS)
				}
			}
		}
	}

EventsLeft:
	for len(actualEvents) != 0 {
		event := <-actualEvents
		for _, ignorableEvent := range ignoreEvents {
			if event.Type == ignorableEvent {
				continue EventsLeft
			}
		}
		t.Fatalf("extra event occured: %v", event.Type)
	}
}

func nextValidEvent(t *testing.T, events chan *event.PoolEvent, ignoreEvents []string) *event.PoolEvent {
	t.Helper()
NextEvent:
	for {
		if len(events) == 0 {
			t.Fatalf("unable to get next event. too few events occured")
		}

		event := <-events
		for _, Type := range ignoreEvents {
			if event.Type == Type {
				continue NextEvent
			}
		}
		return event
	}
}

func runOperation(t *testing.T, operation map[string]interface{}, testInfo *testInfo, s *Server, checkOutTimeout int32) error {
	threadName, ok := operation["thread"]
	if ok { // to be run in background thread
		testInfo.Lock()
		thread, ok := testInfo.threads[threadName.(string)]
		if !ok {
			thread = &simThread{
				JobQueue: make(chan func(), 200),
			}
			testInfo.threads[threadName.(string)] = thread

			go func() {
				for {
					job, more := <-thread.JobQueue
					if !more {
						break
					}
					job()
					atomic.AddInt32(&thread.JobsCompleted, 1)
				}
			}()
		}
		testInfo.Unlock()

		atomic.AddInt32(&thread.JobsAssigned, 1)
		thread.JobQueue <- func() {
			err := runOperationInThread(t, operation, testInfo, s, checkOutTimeout)
			testInfo.backgroundThreadErrors <- err
		}

		return nil // since we don't care about errors occurring in non primary threads
	}
	return runOperationInThread(t, operation, testInfo, s, checkOutTimeout)
}

func runOperationInThread(t *testing.T, operation map[string]interface{}, testInfo *testInfo, s *Server, checkOutTimeout int32) error {
	name, ok := operation["name"]
	if !ok {
		t.Fatalf("unable to find name in operation")
	}

	switch name {
	case "start":
		return nil // we dont need to start another thread since this has already been done in runOperation
	case "wait":
		timeMs, ok := operation["ms"]
		if !ok {
			t.Fatalf("unable to find ms in wait operation")
		}
		dur := time.Duration(int64(timeMs.(float64))) * time.Millisecond
		time.Sleep(dur)
	case "waitForThread":
		threadName, ok := operation["target"]
		if !ok {
			t.Fatalf("unable to waitForThread without specified threadName")
		}

		testInfo.Lock()
		thread, ok := testInfo.threads[threadName.(string)]
		testInfo.Unlock()
		if !ok {
			t.Fatalf("unable to find thread to wait for: %v", threadName)
		}

		for {
			if atomic.LoadInt32(&thread.JobsCompleted) == atomic.LoadInt32(&thread.JobsAssigned) {
				break
			}
		}
	case "waitForEvent":
		targetCount, ok := operation["count"]
		if !ok {
			t.Fatalf("count is required to waitForEvent")
		}
		targetEventName, ok := operation["event"]
		if !ok {
			t.Fatalf("event is require to waitForEvent")
		}

		originalChan := testInfo.originalEventChan
		finalChan := testInfo.finalEventChan

		for {
			event := <-originalChan
			finalChan <- event

			testInfo.Lock()
			_, ok = testInfo.eventCounts[event.Type]
			if !ok {
				testInfo.eventCounts[event.Type] = 0
			}
			testInfo.eventCounts[event.Type]++
			count := testInfo.eventCounts[event.Type]
			testInfo.Unlock()

			if event.Type == targetEventName.(string) && count == uint64(targetCount.(float64)) {
				break
			}
		}
	case "checkOut":
		checkoutContext := context.Background()
		if checkOutTimeout != 0 {
			var cancel context.CancelFunc
			checkoutContext, cancel = context.WithTimeout(context.Background(), time.Duration(checkOutTimeout)*time.Millisecond)
			defer cancel()
		}

		c, err := s.Connection(checkoutContext)
		if label, ok := operation["label"]; ok {
			testInfo.Lock()
			testInfo.objects[label.(string)] = c
			testInfo.Unlock()
		}

		return err
	case "checkIn":
		cName, ok := operation["connection"]
		if !ok {
			t.Fatalf("unable to find connection to checkin")
		}

		var cEmptyInterface interface{}
		testInfo.Lock()
		cEmptyInterface, ok = testInfo.objects[cName.(string)]
		delete(testInfo.objects, cName.(string))
		testInfo.Unlock()
		if !ok {
			t.Fatalf("was unable to find %v in objects when expected", cName)
		}

		c, ok := cEmptyInterface.(*Connection)
		if !ok {
			t.Fatalf("object in objects was expected to be a connection, but was instead a %T", cEmptyInterface)
		}
		return c.Close()
	case "clear":
		s.pool.clear()
	case "close":
		return s.pool.disconnect(context.Background())
	default:
		t.Fatalf("unknown operation: %v", name)
	}

	return nil
}
