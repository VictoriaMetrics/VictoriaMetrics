package ec2

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestParseMetadataSecurityCredentialsFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		creds, err := parseMetadataSecurityCredentials([]byte(s))
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if creds != nil {
			t.Fatalf("expecting nil apiCreds; got %v", creds)
		}
	}
	f("")
	f("foobar")
}

func TestParseMetadataSecurityCredentialsSuccess(t *testing.T) {
	// See https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html
	s := `{
  "Code" : "Success",
  "LastUpdated" : "2012-04-26T16:39:16Z",
  "Type" : "AWS-HMAC",
  "AccessKeyId" : "ASIAIOSFODNN7EXAMPLE",
  "SecretAccessKey" : "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
  "Token" : "token",
  "Expiration" : "2017-05-17T15:09:54Z"
}`
	creds, err := parseMetadataSecurityCredentials([]byte(s))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	credsExpected := &apiCredentials{
		AccessKeyID:     "ASIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		Token:           "token",
		Expiration:      mustParseRFC3339("2017-05-17T15:09:54Z"),
	}
	if !reflect.DeepEqual(creds, credsExpected) {
		t.Fatalf("unexpected creds;\ngot\n%+v\nwant\n%+v", creds, credsExpected)
	}
}

func TestParseARNCredentialsFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		creds, err := parseARNCredentials([]byte(s))
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if creds != nil {
			t.Fatalf("expecting nil apiCreds; got %v", creds)
		}
	}
	f("")
	f("foobar")
}

func TestParseARNCredentialsSuccess(t *testing.T) {
	// See https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html
	s := `<AssumeRoleResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <AssumeRoleResult>
    <AssumedRoleUser>
      <Arn>arn:aws:sts::123456789012:assumed-role/demo/TestAR</Arn>
      <AssumedRoleId>ARO123EXAMPLE123:TestAR</AssumedRoleId>
    </AssumedRoleUser>
    <Credentials>
      <AccessKeyId>ASIAIOSFODNN7EXAMPLE</AccessKeyId>
      <SecretAccessKey>wJalrXUtnFEMI/K7MDENG/bPxRfiCYzEXAMPLEKEY</SecretAccessKey>
      <SessionToken>
       AQoDYXdzEPT//////////wEXAMPLEtc764bNrC9SAPBSM22wDOk4x4HIZ8j4FZTwdQW
       LWsKWHGBuFqwAeMicRXmxfpSPfIeoIYRqTflfKD8YUuwthAx7mSEI/qkPpKPi/kMcGd
       QrmGdeehM4IC1NtBmUpp2wUE8phUZampKsburEDy0KPkyQDYwT7WZ0wq5VSXDvp75YU
       9HFvlRd8Tx6q6fE8YQcHNVXAkiY9q6d+xo0rKwT38xVqr7ZD0u0iPPkUL64lIZbqBAz
       +scqKmlzm8FDrypNC9Yjc8fPOLn9FX9KSYvKTr4rvx3iSIlTJabIQwj2ICCR/oLxBA==
      </SessionToken>
      <Expiration>2019-11-09T13:34:41Z</Expiration>
    </Credentials>
    <PackedPolicySize>6</PackedPolicySize>
  </AssumeRoleResult>
  <ResponseMetadata>
    <RequestId>c6104cbe-af31-11e0-8154-cbc7ccf896c7</RequestId>
  </ResponseMetadata>
</AssumeRoleResponse>
`
	creds, err := parseARNCredentials([]byte(s))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	credsExpected := &apiCredentials{
		AccessKeyID:     "ASIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYzEXAMPLEKEY",
		Token: `
       AQoDYXdzEPT//////////wEXAMPLEtc764bNrC9SAPBSM22wDOk4x4HIZ8j4FZTwdQW
       LWsKWHGBuFqwAeMicRXmxfpSPfIeoIYRqTflfKD8YUuwthAx7mSEI/qkPpKPi/kMcGd
       QrmGdeehM4IC1NtBmUpp2wUE8phUZampKsburEDy0KPkyQDYwT7WZ0wq5VSXDvp75YU
       9HFvlRd8Tx6q6fE8YQcHNVXAkiY9q6d+xo0rKwT38xVqr7ZD0u0iPPkUL64lIZbqBAz
       +scqKmlzm8FDrypNC9Yjc8fPOLn9FX9KSYvKTr4rvx3iSIlTJabIQwj2ICCR/oLxBA==
      `,
		Expiration: mustParseRFC3339("2019-11-09T13:34:41Z"),
	}
	if !reflect.DeepEqual(creds, credsExpected) {
		t.Fatalf("unexpected creds;\ngot\n%+v\nwant\n%+v", creds, credsExpected)
	}
}

func mustParseRFC3339(s string) time.Time {
	expTime, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(fmt.Errorf("unexpected error when parsing time from %q: %w", s, err))
	}
	return expTime
}
