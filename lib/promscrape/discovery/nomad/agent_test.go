package nomad

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
		"config": {
		  "ACL": {
			"Enabled": false,
			"PolicyTTL": 30000000000,
			"ReplicationToken": "",
			"RoleTTL": 30000000000,
			"TokenMaxExpirationTTL": 0,
			"TokenMinExpirationTTL": 0,
			"TokenTTL": 30000000000
		  },
		  "Addresses": {
			"HTTP": "0.0.0.0",
			"RPC": "0.0.0.0",
			"Serf": "0.0.0.0"
		  },
		  "AdvertiseAddrs": {
			"HTTP": "192.168.29.76:4646",
			"RPC": "192.168.29.76:4647",
			"Serf": "192.168.29.76:4648"
		  },
		  "Audit": {
			"Enabled": null,
			"Filters": null,
			"Sinks": null
		  },
		  "Autopilot": {
			"CleanupDeadServers": null,
			"DisableUpgradeMigration": null,
			"EnableCustomUpgrades": null,
			"EnableRedundancyZones": null,
			"LastContactThreshold": 200000000,
			"MaxTrailingLogs": 250,
			"MinQuorum": 0,
			"ServerStabilizationTime": 10000000000
		  },
		  "BindAddr": "0.0.0.0",
		  "Client": {
			"AllocDir": "",
			"Artifact": {
			  "GCSTimeout": "30m",
			  "GitTimeout": "30m",
			  "HTTPMaxSize": "100GB",
			  "HTTPReadTimeout": "30m",
			  "HgTimeout": "30m",
			  "S3Timeout": "30m"
			},
			"BindWildcardDefaultHostNetwork": true,
			"BridgeNetworkName": "",
			"BridgeNetworkSubnet": "",
			"CNIConfigDir": "/opt/cni/config",
			"CNIPath": "/opt/cni/bin",
			"CgroupParent": "",
			"ChrootEnv": {
			  "/run/resolvconf": "/run/resolvconf",
			  "/sbin": "/sbin",
			  "/usr": "/usr",
			  "/bin/": "/bin/",
			  "/etc/": "/etc/",
			  "/lib": "/lib",
			  "/lib32": "/lib32",
			  "/lib64": "/lib64"
			},
			"ClientMaxPort": 14512,
			"ClientMinPort": 14000,
			"CpuCompute": 0,
			"DisableRemoteExec": false,
			"Enabled": true,
			"GCDiskUsageThreshold": 80,
			"GCInodeUsageThreshold": 70,
			"GCInterval": 60000000000,
			"GCMaxAllocs": 50,
			"GCParallelDestroys": 2,
			"HostNetworks": null,
			"HostVolumes": null,
			"MaxDynamicPort": 32000,
			"MaxKillTimeout": "30s",
			"MemoryMB": 0,
			"Meta": {
			  "stack": "zerodha",
			  "env": "dev"
			},
			"MinDynamicPort": 20000,
			"NetworkInterface": "",
			"NetworkSpeed": 0,
			"NoHostUUID": true,
			"NodeClass": "",
			"NomadServiceDiscovery": true,
			"Options": {},
			"ReserveableCores": "",
			"Reserved": {
			  "CPU": 0,
			  "Cores": "2",
			  "DiskMB": 1024,
			  "MemoryMB": 1024,
			  "ReservedPorts": "22"
			},
			"ServerJoin": {
			  "RetryInterval": 30000000000,
			  "RetryJoin": [],
			  "RetryMaxAttempts": 0,
			  "StartJoin": null
			},
			"Servers": null,
			"StateDir": "",
			"TemplateConfig": {
			  "BlockQueryWaitTime": null,
			  "BlockQueryWaitTimeHCL": "",
			  "ConsulRetry": null,
			  "DisableSandbox": false,
			  "FunctionBlacklist": null,
			  "FunctionDenylist": [
				"plugin",
				"writeToFile"
			  ],
			  "MaxStale": null,
			  "MaxStaleHCL": "",
			  "NomadRetry": null,
			  "VaultRetry": null
			}
		  },
		  "Consul": {
			"Addr": "127.0.0.1:8500",
			"AllowUnauthenticated": true,
			"Auth": "",
			"AutoAdvertise": true,
			"CAFile": "",
			"CertFile": "",
			"ChecksUseAdvertise": false,
			"ClientAutoJoin": true,
			"ClientHTTPCheckName": "Nomad Client HTTP Check",
			"ClientServiceName": "nomad-client",
			"EnableSSL": false,
			"GRPCAddr": "",
			"KeyFile": "",
			"Namespace": "",
			"ServerAutoJoin": true,
			"ServerHTTPCheckName": "Nomad Server HTTP Check",
			"ServerRPCCheckName": "Nomad Server RPC Check",
			"ServerSerfCheckName": "Nomad Server Serf Check",
			"ServerServiceName": "nomad",
			"ShareSSL": null,
			"Tags": null,
			"Timeout": 5000000000,
			"Token": "",
			"VerifySSL": true
		  },
		  "DataDir": "/opt/nomad/data",
		  "Datacenter": "dc1",
		  "DevMode": false,
		  "DisableAnonymousSignature": false,
		  "DisableUpdateCheck": false,
		  "EnableDebug": false,
		  "EnableSyslog": false,
		  "Files": [
			"nomad.hcl"
		  ],
		  "HTTPAPIResponseHeaders": {},
		  "LeaveOnInt": false,
		  "LeaveOnTerm": false,
		  "Limits": {
			"HTTPMaxConnsPerClient": 100,
			"HTTPSHandshakeTimeout": "5s",
			"RPCHandshakeTimeout": "5s",
			"RPCMaxConnsPerClient": 100
		  },
		  "LogFile": "",
		  "LogJson": false,
		  "LogLevel": "INFO",
		  "LogRotateBytes": 0,
		  "LogRotateDuration": "",
		  "LogRotateMaxFiles": 0,
		  "NodeName": "",
		  "PluginDir": "/opt/nomad/data/plugins",
		  "Plugins": [
			{
			  "Args": null,
			  "Config": {
				"allow_privileged": true,
				"volumes": [
				  {
					"enabled": true
				  }
				],
				"extra_labels": [
				  "job_name",
				  "job_id",
				  "task_group_name",
				  "task_name",
				  "namespace",
				  "node_name",
				  "node_id"
				]
			  },
			  "Name": "docker"
			},
			{
			  "Args": null,
			  "Config": {
				"enabled": true,
				"no_cgroups": true
			  },
			  "Name": "raw_exec"
			}
		  ],
		  "Ports": {
			"HTTP": 4646,
			"RPC": 4647,
			"Serf": 4648
		  },
		  "Region": "global",
		  "Sentinel": {
			"Imports": null
		  },
		  "Server": {
			"ACLTokenGCThreshold": "",
			"AuthoritativeRegion": "",
			"BootstrapExpect": 1,
			"CSIPluginGCThreshold": "",
			"CSIVolumeClaimGCThreshold": "",
			"DataDir": "",
			"DefaultSchedulerConfig": null,
			"DeploymentGCThreshold": "",
			"DeploymentQueryRateLimit": 0,
			"EnableEventBroker": true,
			"Enabled": true,
			"EnabledSchedulers": null,
			"EvalGCThreshold": "",
			"EventBufferSize": 100,
			"FailoverHeartbeatTTL": 0,
			"HeartbeatGrace": 0,
			"JobGCInterval": "",
			"JobGCThreshold": "",
			"LicenseEnv": "",
			"LicensePath": "",
			"MaxHeartbeatsPerSecond": 0,
			"MinHeartbeatTTL": 0,
			"NodeGCThreshold": "",
			"NonVotingServer": false,
			"NumSchedulers": null,
			"PlanRejectionTracker": {
			  "Enabled": false,
			  "NodeThreshold": 100,
			  "NodeWindow": 300000000000
			},
			"RaftBoltConfig": null,
			"RaftMultiplier": null,
			"RaftProtocol": 3,
			"RedundancyZone": "",
			"RejoinAfterLeave": false,
			"RetryInterval": 0,
			"RetryJoin": [],
			"RetryMaxAttempts": 0,
			"RootKeyGCInterval": "",
			"RootKeyGCThreshold": "",
			"RootKeyRotationThreshold": "",
			"Search": {
			  "FuzzyEnabled": true,
			  "LimitQuery": 20,
			  "LimitResults": 100,
			  "MinTermLength": 2
			},
			"ServerJoin": {
			  "RetryInterval": 30000000000,
			  "RetryJoin": [],
			  "RetryMaxAttempts": 0,
			  "StartJoin": null
			},
			"StartJoin": [],
			"UpgradeVersion": ""
		  },
		  "SyslogFacility": "LOCAL0",
		  "TLSConfig": {
			"CAFile": "",
			"CertFile": "",
			"Checksum": "",
			"EnableHTTP": false,
			"EnableRPC": false,
			"KeyFile": "",
			"KeyLoader": {},
			"RPCUpgradeMode": false,
			"TLSCipherSuites": "",
			"TLSMinVersion": "",
			"TLSPreferServerCipherSuites": false,
			"VerifyHTTPSClient": false,
			"VerifyServerHostname": false
		  },
		  "Telemetry": {
			"CirconusAPIApp": "",
			"CirconusAPIToken": "",
			"CirconusAPIURL": "",
			"CirconusBrokerID": "",
			"CirconusBrokerSelectTag": "",
			"CirconusCheckDisplayName": "",
			"CirconusCheckForceMetricActivation": "",
			"CirconusCheckID": "",
			"CirconusCheckInstanceID": "",
			"CirconusCheckSearchTag": "",
			"CirconusCheckSubmissionURL": "",
			"CirconusCheckTags": "",
			"CirconusSubmissionInterval": "",
			"CollectionInterval": "1s",
			"DataDogAddr": "",
			"DataDogTags": null,
			"DisableDispatchedJobSummaryMetrics": false,
			"DisableHostname": false,
			"FilterDefault": null,
			"PrefixFilter": null,
			"PrometheusMetrics": false,
			"PublishAllocationMetrics": false,
			"PublishNodeMetrics": false,
			"StatsdAddr": "",
			"StatsiteAddr": "",
			"UseNodeName": false
		  },
		  "UI": {
			"Consul": {
			  "BaseUIURL": ""
			},
			"Enabled": true,
			"Vault": {
			  "BaseUIURL": ""
			}
		  },
		  "Vault": {
			"Addr": "https://vault.service.consul:8200",
			"AllowUnauthenticated": true,
			"ConnectionRetryIntv": 30000000000,
			"Enabled": null,
			"Namespace": "",
			"Role": "",
			"TLSCaFile": "",
			"TLSCaPath": "",
			"TLSCertFile": "",
			"TLSKeyFile": "",
			"TLSServerName": "",
			"TLSSkipVerify": null,
			"TaskTokenTTL": "",
			"Token": ""
		  },
		  "Version": {
			"Revision": "f464aca721d222ae9c1f3df643b3c3aaa20e2da7",
			"Version": "1.4.3",
			"VersionMetadata": "",
			"VersionPrerelease": ""
		  }
		},
		"member": {
		  "Addr": "192.168.29.76",
		  "DelegateCur": 4,
		  "DelegateMax": 5,
		  "DelegateMin": 2,
		  "Name": "pop-os.global",
		  "Port": 4648,
		  "ProtocolCur": 2,
		  "ProtocolMax": 5,
		  "ProtocolMin": 1,
		  "Status": "alive",
		  "Tags": {
			"rpc_addr": "192.168.29.76",
			"bootstrap": "1",
			"expect": "1",
			"role": "nomad",
			"region": "global",
			"build": "1.4.3",
			"revision": "f464aca721d222ae9c1f3df643b3c3aaa20e2da7",
			"port": "4647",
			"vsn": "1",
			"dc": "dc1",
			"raft_vsn": "3",
			"id": "d78cdda9-7e35-48d0-a0e0-36041f6df0ec"
		  }
		},
		"stats": {
		  "nomad": {
			"leader": "true",
			"leader_addr": "192.168.29.76:4647",
			"bootstrap": "true",
			"known_regions": "1",
			"server": "true"
		  },
		  "raft": {
			"term": "3",
			"latest_configuration_index": "0",
			"applied_index": "384",
			"protocol_version_min": "0",
			"protocol_version_max": "3",
			"snapshot_version_min": "0",
			"num_peers": "0",
			"state": "Leader",
			"last_log_term": "3",
			"commit_index": "384",
			"last_contact": "0",
			"last_snapshot_index": "0",
			"protocol_version": "3",
			"snapshot_version_max": "1",
			"latest_configuration": "[{Suffrage:Voter ID:d78cdda9-7e35-48d0-a0e0-36041f6df0ec Address:172.20.10.3:4647}]",
			"last_log_index": "384",
			"fsm_pending": "0",
			"last_snapshot_term": "0"
		  },
		  "serf": {
			"coordinate_resets": "0",
			"health_score": "0",
			"member_time": "1",
			"query_time": "1",
			"intent_queue": "0",
			"event_queue": "0",
			"encrypted": "false",
			"members": "1",
			"failed": "0",
			"left": "0",
			"event_time": "1",
			"query_queue": "0"
		  },
		  "runtime": {
			"arch": "amd64",
			"version": "go1.19.3",
			"max_procs": "8",
			"goroutines": "294",
			"cpu_count": "8",
			"kernel.name": "linux"
		  },
		  "vault": {
			"tracked_for_revoked": "0",
			"token_ttl": "0s",
			"token_expire_time": "",
			"token_last_renewal_time": "",
			"token_next_renewal_time": ""
		  },
		  "client": {
			"node_id": "9e02c85b-db59-45f1-ddee-40d0317bd33d",
			"known_servers": "192.168.29.76:4647",
			"num_allocations": "3",
			"last_heartbeat": "13.254411467s",
			"heartbeat_ttl": "19.86001456s"
		  }
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
