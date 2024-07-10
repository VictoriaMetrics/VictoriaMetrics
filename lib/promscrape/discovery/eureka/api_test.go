package eureka

import (
	"reflect"
	"testing"
)

func TestParseAPIResponse(t *testing.T) {
	f := func(data string, resultExpected *applications) {
		t.Helper()

		result, err := parseAPIResponse([]byte(data))
		if err != nil {
			t.Fatalf("parseAPIResponse() error: %s", err)
		}
		if !reflect.DeepEqual(result, resultExpected) {
			t.Fatalf("unexpected result\ngot\n%v\nwant\n%v", result, resultExpected)
		}
	}

	// parse ok 1 app with instance
	data := `<applications>
  <versions__delta>1</versions__delta>
  <apps__hashcode>UP_1_</apps__hashcode>
  <application>
    <name>HELLO-NETFLIX-OSS</name>
    <instance>
      <hostName>98de25ebef42</hostName>
      <app>HELLO-NETFLIX-OSS</app>
      <ipAddr>10.10.0.3</ipAddr>
      <status>UP</status>
      <overriddenstatus>UNKNOWN</overriddenstatus>
      <port enabled="true">8080</port>
      <securePort enabled="false">443</securePort>
      <countryId>1</countryId>
      <dataCenterInfo class="com.netflix.appinfo.InstanceInfo$DefaultDataCenterInfo">
        <name>MyOwn</name>
      </dataCenterInfo>
      <leaseInfo>
        <renewalIntervalInSecs>30</renewalIntervalInSecs>
        <durationInSecs>90</durationInSecs>
        <registrationTimestamp>1605757726477</registrationTimestamp>
        <lastRenewalTimestamp>1605759135484</lastRenewalTimestamp>
        <evictionTimestamp>0</evictionTimestamp>
        <serviceUpTimestamp>1605757725913</serviceUpTimestamp>
      </leaseInfo>
      <metadata class="java.util.Collections$EmptyMap"/>
      <appGroupName>UNKNOWN</appGroupName>
      <homePageUrl>http://98de25ebef42:8080/</homePageUrl>
      <statusPageUrl>http://98de25ebef42:8080/Status</statusPageUrl>
      <healthCheckUrl>http://98de25ebef42:8080/healthcheck</healthCheckUrl>
      <vipAddress>HELLO-NETFLIX-OSS</vipAddress>
      <isCoordinatingDiscoveryServer>false</isCoordinatingDiscoveryServer>
      <lastUpdatedTimestamp>1605757726478</lastUpdatedTimestamp>
      <lastDirtyTimestamp>1605757725753</lastDirtyTimestamp>
      <actionType>ADDED</actionType>
    </instance>
  </application>
</applications>`

	resultExpected := &applications{
		Applications: []Application{
			{
				Name: "HELLO-NETFLIX-OSS",
				Instances: []Instance{
					{
						HostName:         "98de25ebef42",
						HomePageURL:      "http://98de25ebef42:8080/",
						StatusPageURL:    "http://98de25ebef42:8080/Status",
						HealthCheckURL:   "http://98de25ebef42:8080/healthcheck",
						App:              "HELLO-NETFLIX-OSS",
						IPAddr:           "10.10.0.3",
						VipAddress:       "HELLO-NETFLIX-OSS",
						SecureVipAddress: "",
						Status:           "UP",
						Port: Port{
							Enabled: true,
							Port:    8080,
						},
						SecurePort: Port{
							Port: 443,
						},
						DataCenterInfo: DataCenterInfo{
							Name: "MyOwn",
						},
						Metadata:   MetaData{},
						CountryID:  1,
						InstanceID: "",
					},
				},
			},
		},
	}
	f(data, resultExpected)
}
