package puppetdb

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

var jsonResponse = `[
   {
      "certname": "edinburgh.example.com",
      "environment": "prod",
      "exported": false,
      "file": "/etc/puppetlabs/code/environments/prod/modules/upstream/apache/manifests/init.pp",
      "line": 384,
      "parameters": {
         "access_log": true,
         "access_log_file": "ssl_access_log",
         "additional_includes": [ ],
         "directoryindex": "",
         "docroot": "/var/www/html",
         "ensure": "absent",
         "options": [
            "Indexes",
            "FollowSymLinks",
            "MultiViews"
         ],
         "php_flags": { },
         "labels": {
            "alias": "edinburgh"
         },
         "scriptaliases": [
            {
               "alias": "/cgi-bin",
               "path": "/var/www/cgi-bin"
            }
         ],
         "port": 22,
         "pi": 3.141592653589793,
         "buckets": [
            0,
            2,
            5
         ],
         "coordinates": [
            60.13464726551357,
            -2.0513768021728893
         ]
      },
      "resource": "49af83866dc5a1518968b68e58a25319107afe11",
      "tags": [
         "roles::hypervisor",
         "apache",
         "apache::vhost",
         "class",
         "default-ssl",
         "profile_hypervisor",
         "vhost",
         "profile_apache",
         "hypervisor",
         "__node_regexp__edinburgh",
         "roles",
         "node"
      ],
      "title": "default-ssl",
      "type": "Apache::Vhost"
   }
]`

// TestSDConfig_GetLabels test example response and expect labels are from:
// https://github.com/prometheus/prometheus/blob/685493187ec5f5734777769f595cf8418d49900d/discovery/puppetdb/puppetdb_test.go#L110C6-L110C39
func TestSDConfig_GetLabels(t *testing.T) {
	mockSvr := newMockPuppetDBServer(func(_ string) ([]byte, error) {
		return []byte(jsonResponse), nil
	})

	sdConfig := &SDConfig{
		URL:               mockSvr.URL,
		Query:             "vhosts",
		Port:              9100,
		IncludeParameters: true,
	}

	expectLabels := &promutils.Labels{}
	expectLabels.Add("__address__", "edinburgh.example.com:9100")
	expectLabels.Add("__meta_puppetdb_query", "vhosts")
	expectLabels.Add("__meta_puppetdb_certname", "edinburgh.example.com")
	expectLabels.Add("__meta_puppetdb_environment", "prod")
	expectLabels.Add("__meta_puppetdb_exported", "false")
	expectLabels.Add("__meta_puppetdb_file", "/etc/puppetlabs/code/environments/prod/modules/upstream/apache/manifests/init.pp")
	expectLabels.Add("__meta_puppetdb_parameter_access_log", "true")
	expectLabels.Add("__meta_puppetdb_parameter_access_log_file", "ssl_access_log")
	expectLabels.Add("__meta_puppetdb_parameter_buckets", "0,2,5")
	expectLabels.Add("__meta_puppetdb_parameter_coordinates", "60.13464726551357,-2.0513768021728893")
	expectLabels.Add("__meta_puppetdb_parameter_docroot", "/var/www/html")
	expectLabels.Add("__meta_puppetdb_parameter_ensure", "absent")
	expectLabels.Add("__meta_puppetdb_parameter_labels_alias", "edinburgh")
	expectLabels.Add("__meta_puppetdb_parameter_options", "Indexes,FollowSymLinks,MultiViews")
	expectLabels.Add("__meta_puppetdb_parameter_pi", "3.141592653589793")
	expectLabels.Add("__meta_puppetdb_parameter_port", "22")
	expectLabels.Add("__meta_puppetdb_resource", "49af83866dc5a1518968b68e58a25319107afe11")
	expectLabels.Add("__meta_puppetdb_tags", ",roles::hypervisor,apache,apache::vhost,class,default-ssl,profile_hypervisor,vhost,profile_apache,hypervisor,__node_regexp__edinburgh,roles,node,")
	expectLabels.Add("__meta_puppetdb_title", "default-ssl")
	expectLabels.Add("__meta_puppetdb_type", "Apache::Vhost")

	result, err := sdConfig.GetLabels("")
	if err != nil {
		t.Fatalf("GetLabels got err: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("GetLabels get result len != 1")
	}

	if !reflect.DeepEqual(result[0].ToMap(), expectLabels.ToMap()) {
		t.Fatalf("GetLabels incorrect, want: %v, got: %v", expectLabels.ToMap(), result[0].ToMap())
	}
}
