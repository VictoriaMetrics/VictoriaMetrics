# App Integration Tests

The `apptest` package contains the integration tests for the VictoriaMetrics
applications (such as vmstorage, vminsert, and vmselect).

An integration test aims at verifying the behavior of an application as a whole,
as apposed to a unit test that verifies the behavior of a building block of an
application.

To achieve that an integration test starts an application in a separate process
and then issues HTTP requests to it and verifies the responses, examines the
metrics the app exposes and/or files it creates, etc.

Note that an object of testing may be not just a single app, but several apps
working together. A good example is VictoriaMetrics cluster. An integration test
may reproduce an arbitrary cluster configuration and verify how the components
work together as a system.

The package provides a collection of helpers to start applications and make
queries to them:

-   `app.go` - contains the generic code for staring an application and should
    not be used by integration tests directly.
-   `{vmstorage,vminsert,etc}.go` - build on top of `app.go` and provide the
    code for staring a specific application.
-   `client.go` - provides helper functions for sending HTTP requests to
    applications.

The integration tests themselves reside in `tests/*_test.go` files. Apart from having
the `_test` suffix, there are no strict rules of how to name a file, but the
name should reflect the prevailing purpose of the tests located in that file.
For example, `sharding_test.go` aims at testing data sharding.

Since integration tests start applications in a separate process, they require
the application binary files to be built and put into the `bin` directory. The
build rule used for running integration tests, `make integration-test`,
accounts for that, it builds all application binaries before running the tests.
But if you want to run the tests without `make`, i.e. by executing
`go test ./app/apptest`, you will need to build the binaries first (for example,
by executing `make all`).

Not all binaries can be built from `master` branch, cluster binaries can be built
only from `cluster` branch. Hence, not all test cases suitable to run in both branches:
- If test is using binaries from `cluster` branch, then test name should be prefixed 
  with `TestCluster` word
- If test is using binaries from `master` branch, then test name should be prefixed
  with `TestSingle` word.
