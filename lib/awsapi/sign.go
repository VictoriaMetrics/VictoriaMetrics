package awsapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// for get requests there is no need to calculate payload hash each time.
var emptyPayloadHash = hashHex("")

// newSignedGetRequest creates signed http get request for apiURL according to aws signature algorithm.
//
// See the algorithm at https://docs.aws.amazon.com/general/latest/gr/sigv4-signed-request-examples.html
func newSignedGetRequest(apiURL, service, region string, creds *credentials) (*http.Request, error) {
	return newSignedGetRequestWithTime(apiURL, service, region, creds, time.Now().UTC())
}

func newSignedGetRequestWithTime(apiURL, service, region string, creds *credentials, t time.Time) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create http request with given apiURL: %s, err: %w", apiURL, err)
	}
	if err := signRequestWithTime(req, service, region, emptyPayloadHash, creds, t); err != nil {
		return nil, err
	}
	return req, nil
}

// signRequestWithTime - signs http request with AWS API credentials for given payload
func signRequestWithTime(req *http.Request, service, region, payloadHash string, creds *credentials, t time.Time) error {
	uri := req.URL
	// Create canonicalRequest
	amzdate := t.Format("20060102T150405Z")
	datestamp := t.Format("20060102")
	canonicalURL := uri.Path
	canonicalQS := uri.Query().Encode()
	// Replace "%20" with "+" according to AWS requirements.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3171
	canonicalQS = strings.ReplaceAll(canonicalQS, "+", "%20")

	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-date:%s\n", uri.Host, amzdate)
	signedHeaders := "host;x-amz-date"
	tmp := []string{
		req.Method,
		canonicalURL,
		canonicalQS,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}
	canonicalRequest := strings.Join(tmp, "\n")

	algorithm := "AWS4-HMAC-SHA256"
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", datestamp, region, service)
	tmp = []string{
		algorithm,
		amzdate,
		credentialScope,
		hashHex(canonicalRequest),
	}
	stringToSign := strings.Join(tmp, "\n")

	// Calculate the signature
	signingKey := getSignatureKey(creds.SecretAccessKey, datestamp, region, service)
	signature := hmacHex(signingKey, stringToSign)

	// Calculate autheader
	authHeader := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s", algorithm, creds.AccessKeyID, credentialScope, signedHeaders, signature)

	req.Header.Set("x-amz-date", amzdate)
	req.Header.Set("Authorization", authHeader)
	// special case for token auth
	if creds.Token != "" {
		req.Header.Set("X-Amz-Security-Token", creds.Token)
	}
	return nil
}

func getSignatureKey(key, datestamp, region, service string) string {
	dateKey := hmacBin("AWS4"+key, datestamp)
	regionKey := hmacBin(dateKey, region)
	serviceKey := hmacBin(regionKey, service)
	return hmacBin(serviceKey, "aws4_request")
}

func hashHex(s string) string {
	return HashHex([]byte(s))
}

// HashHex hashes given s
func HashHex(s []byte) string {
	h := sha256.Sum256(s)
	return hex.EncodeToString(h[:])
}

func hmacHex(key, data string) string {
	digest := hmacBin(key, data)
	return hex.EncodeToString([]byte(digest))
}

func hmacBin(key, data string) string {
	h := hmac.New(sha256.New, []byte(key))
	_, err := h.Write([]byte(data))
	if err != nil {
		logger.Panicf("BUG: unexpected error when writing to hmac: %s", err)
	}
	return string(h.Sum(nil))
}
