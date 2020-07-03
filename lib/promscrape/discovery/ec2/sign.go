package ec2

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// newSignedRequest signed request for apiURL according to aws signature algorithm.
//
// See the algorithm at https://docs.aws.amazon.com/general/latest/gr/sigv4-signed-request-examples.html
func newSignedRequest(apiURL, service, region, accessKey, secretKey string) (*http.Request, error) {
	t := time.Now().UTC()
	return newSignedRequestWithTime(apiURL, service, region, accessKey, secretKey, t)
}

func newSignedRequestWithTime(apiURL, service, region, accessKey, secretKey string, t time.Time) (*http.Request, error) {
	uri, err := url.Parse(apiURL)
	if err != nil {
		return nil, fmt.Errorf("cannot parse %q: %w", apiURL, err)
	}

	// Create canonicalRequest
	amzdate := t.Format("20060102T150405Z")
	datestamp := t.Format("20060102")
	canonicalURL := uri.Path
	canonicalQS := uri.Query().Encode()
	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-date:%s\n", uri.Host, amzdate)
	signedHeaders := "host;x-amz-date"
	payloadHash := hashHex("")
	tmp := []string{
		"GET",
		canonicalURL,
		canonicalQS,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}
	canonicalRequest := strings.Join(tmp, "\n")

	// Create stringToSign
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
	signingKey := getSignatureKey(secretKey, datestamp, region, service)
	signature := hmacHex(signingKey, stringToSign)

	// Calculate autheader
	authHeader := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s", algorithm, accessKey, credentialScope, signedHeaders, signature)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request from %q: %w", apiURL, err)
	}
	req.Header.Set("x-amz-date", amzdate)
	req.Header.Set("Authorization", authHeader)
	return req, nil
}

func getSignatureKey(key, datestamp, region, service string) string {
	kDate := hmacBin("AWS4"+key, datestamp)
	kRegion := hmacBin(kDate, region)
	kService := hmacBin(kRegion, service)
	return hmacBin(kService, "aws4_request")
}

func hashHex(s string) string {
	h := sha256.Sum256([]byte(s))
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
