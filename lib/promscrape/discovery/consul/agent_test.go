package consul

import (
	"reflect"
	"testing"
)

func TestParseAgentFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		a, err := parseAgent([]byte(s))
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if a != nil {
			t.Fatalf("unexpected non-nil Agent: %v", a)
		}
	}
	f(``)
	f(`[1,23]`)
}

func TestParseAgentSuccess(t *testing.T) {
	data := `
{
  "Config": {
    "Datacenter": "dc1",
    "NodeName": "foobar",
    "NodeID": "9d754d17-d864-b1d3-e758-f3fe25a9874f",
    "Server": true,
    "Revision": "deadbeef",
    "Version": "1.0.0"
  },
  "DebugConfig": {
  },
  "Coord": {
    "Adjustment": 0,
    "Error": 1.5,
    "Vec": [0,0,0,0,0,0,0,0]
  },
  "Member": {
    "Name": "foobar",
    "Addr": "10.1.10.12",
    "Port": 8301,
    "Tags": {
      "bootstrap": "1",
      "dc": "dc1",
      "id": "40e4a748-2192-161a-0510-9bf59fe950b5",
      "port": "8300",
      "role": "consul",
      "vsn": "1",
      "vsn_max": "1",
      "vsn_min": "1"
    },
    "Status": 1,
    "ProtocolMin": 1,
    "ProtocolMax": 2,
    "ProtocolCur": 2,
    "DelegateMin": 2,
    "DelegateMax": 4,
    "DelegateCur": 4
  },
  "Meta": {
    "instance_type": "i2.xlarge",
    "os_version": "ubuntu_16.04"
  }
}
`
	a, err := parseAgent([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	aExpected := &Agent{
		Config: AgentConfig{
			Datacenter: "dc1",
		},
	}
	if !reflect.DeepEqual(a, aExpected) {
		t.Fatalf("unexpected Agent parsed;\ngot\n%v\nwant\n%v", a, aExpected)
	}
}
