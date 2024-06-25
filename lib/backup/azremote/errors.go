package azremote

import "errors"

// errNoCredentials is returned when no valid combination of credentials is
// found.
var errNoCredentials = errors.New("failed to detect any credentials")

// errAzureSDKError is used to wrap errors returned by the Azure SDK.
var errAzureSDKError = errors.New("azure sdk error")
