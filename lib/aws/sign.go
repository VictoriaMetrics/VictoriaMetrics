package aws

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// NewSignedRequestWithTime signed request for apiURL according to aws signature algorithm.
//
// See the algorithm at https://docs.aws.amazon.com/general/latest/gr/sigv4-signed-request-examples.html
func NewSignedRequestWithTime(apiURL, service, region string, creds *credentials, t time.Time) (*http.Request, error) {
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create http request with given apiURL: %s, err: %w", apiURL, err)
	}
	if err := signRequestWithTime(req, service, region, creds, t); err != nil {
		return nil, err
	}
	return req, nil
}

// SignRequestWithConfig - signs request with given config for service access.
func SignRequestWithConfig(req *http.Request, cfg *Config, service string) error {
	ac, err := cfg.getFreshAPICredentials()
	if err != nil {
		return err
	}
	return signRequestWithTime(req, service, cfg.region, ac, time.Now().UTC())
}

var bbp bytesutil.ByteBufferPool

// signRequestWithTime - signs given http request.
func signRequestWithTime(req *http.Request, service, region string, creds *credentials, t time.Time) error {
	uri := req.URL
	// Create canonicalRequest
	amzdate := t.Format("20060102T150405Z")
	datestamp := t.Format("20060102")
	canonicalURL := uri.Path
	canonicalQS := uri.Query().Encode()

	var payload string
	if req.Body != nil {
		bb := bbp.Get()
		defer bbp.Put(bb)
		if _, err := io.Copy(bb, req.Body); err != nil {
			return fmt.Errorf("cannot copy request body for sign: %w", err)
		}
		payload = string(bb.B)
		req.Body = ioutil.NopCloser(bytes.NewBuffer(bb.B))
	}

	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-date:%s\n", uri.Host, amzdate)
	signedHeaders := "host;x-amz-date"
	payloadHash := hashHex(payload)
	tmp := []string{
		req.Method,
		canonicalURL,
		canonicalQS,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}
	canonicalRequest := strings.Join(tmp, "\n")
	req.Header = http.Header{}

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
	signingKey := getSignatureKey(creds.SecretAccessKey, datestamp, region, service)
	signature := hmacHex(signingKey, stringToSign)

	// Calculate autheader
	authHeader := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s", algorithm, creds.AccessKeyID, credentialScope, signedHeaders, signature)

	req.Header.Set("x-amz-date", amzdate)
	req.Header.Set("Authorization", authHeader)
	if creds.Token != "" {
		req.Header.Set("X-Amz-Security-Token", creds.Token)
	}
	return nil
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
