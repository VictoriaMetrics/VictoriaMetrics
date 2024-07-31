package prometheus

import (
	"fmt"
	"testing"
)

func BenchmarkAreIdenticalSeriesFast(b *testing.B) {
	b.Run("identical-series-no-timestamps", func(b *testing.B) {
		s := `
# HELP machine_cpu_cores Number of logical CPU cores.
# TYPE machine_cpu_cores gauge
machine_cpu_cores{boot_id="a1b49bdb-4c2a-4943-9ab3-363a316e9260",machine_id="857143c2dbea4a179223627cf9f47d06",system_uuid="03a75ec7-5105-421a-8b8a-3d7190f6e890"} 4
# HELP machine_cpu_physical_cores Number of physical CPU cores.
# TYPE machine_cpu_physical_cores gauge
machine_cpu_physical_cores{boot_id="a1b49bdb-4c2a-4943-9ab3-363a316e9260",machine_id="857143c2dbea4a179223627cf9f47d06",system_uuid="03a75ec7-5105-421a-8b8a-3d7190f6e890"} 2
# HELP machine_cpu_sockets Number of CPU sockets.
# TYPE machine_cpu_sockets gauge
machine_cpu_sockets{boot_id="a1b49bdb-4c2a-4943-9ab3-363a316e9260",machine_id="857143c2dbea4a179223627cf9f47d06",system_uuid="03a75ec7-5105-421a-8b8a-3d7190f6e890"} 1
# HELP machine_memory_bytes Amount of memory installed on the machine.
# TYPE machine_memory_bytes gauge
machine_memory_bytes{boot_id="a1b49bdb-4c2a-4943-9ab3-363a316e9260",machine_id="857143c2dbea4a179223627cf9f47d06",system_uuid="03a75ec7-5105-421a-8b8a-3d7190f6e890"} 1.6706146304e+10
# HELP machine_nvm_avg_power_budget_watts NVM power budget.
# TYPE machine_nvm_avg_power_budget_watts gauge
machine_nvm_avg_power_budget_watts{boot_id="a1b49bdb-4c2a-4943-9ab3-363a316e9260",machine_id="857143c2dbea4a179223627cf9f47d06",system_uuid="03a75ec7-5105-421a-8b8a-3d7190f6e890"} 0
# HELP machine_nvm_capacity NVM capacity value labeled by NVM mode (memory mode or app direct mode).
# TYPE machine_nvm_capacity gauge
machine_nvm_capacity{boot_id="a1b49bdb-4c2a-4943-9ab3-363a316e9260",machine_id="857143c2dbea4a179223627cf9f47d06",mode="app_direct_mode",system_uuid="03a75ec7-5105-421a-8b8a-3d7190f6e890"} 0
machine_nvm_capacity{boot_id="a1b49bdb-4c2a-4943-9ab3-363a316e9260",machine_id="857143c2dbea4a179223627cf9f47d06",mode="memory_mode",system_uuid="03a75ec7-5105-421a-8b8a-3d7190f6e890"} 0
# HELP machine_scrape_error 1 if there was an error while getting machine metrics, 0 otherwise.
# TYPE machine_scrape_error gauge
machine_scrape_error 0
`
		benchmarkAreIdenticalSeriesFast(b, s, s, true)
	})
	b.Run("different-series-no-timestamps", func(b *testing.B) {
		s := `
# HELP machine_cpu_cores Number of logical CPU cores.
# TYPE machine_cpu_cores gauge
machine_cpu_cores{boot_id="a1b49bdb-4c2a-4943-9ab3-363a316e9260",machine_id="857143c2dbea4a179223627cf9f47d06",system_uuid="03a75ec7-5105-421a-8b8a-3d7190f6e890"} 4
# HELP machine_cpu_physical_cores Number of physical CPU cores.
# TYPE machine_cpu_physical_cores gauge
machine_cpu_physical_cores{boot_id="a1b49bdb-4c2a-4943-9ab3-363a316e9260",machine_id="857143c2dbea4a179223627cf9f47d06",system_uuid="03a75ec7-5105-421a-8b8a-3d7190f6e890"} 2
# HELP machine_cpu_sockets Number of CPU sockets.
# TYPE machine_cpu_sockets gauge
machine_cpu_sockets{boot_id="a1b49bdb-4c2a-4943-9ab3-363a316e9260",machine_id="857143c2dbea4a179223627cf9f47d06",system_uuid="03a75ec7-5105-421a-8b8a-3d7190f6e890"} 1
# HELP machine_memory_bytes Amount of memory installed on the machine.
# TYPE machine_memory_bytes gauge
machine_memory_bytes{boot_id="a1b49bdb-4c2a-4943-9ab3-363a316e9260",machine_id="857143c2dbea4a179223627cf9f47d06",system_uuid="03a75ec7-5105-421a-8b8a-3d7190f6e890"} 1.6706146304e+10
# HELP machine_nvm_avg_power_budget_watts NVM power budget.
# TYPE machine_nvm_avg_power_budget_watts gauge
machine_nvm_avg_power_budget_watts{boot_id="a1b49bdb-4c2a-4943-9ab3-363a316e9260",machine_id="857143c2dbea4a179223627cf9f47d06",system_uuid="03a75ec7-5105-421a-8b8a-3d7190f6e890"} 0
# HELP machine_nvm_capacity NVM capacity value labeled by NVM mode (memory mode or app direct mode).
# TYPE machine_nvm_capacity gauge
machine_nvm_capacity{boot_id="a1b49bdb-4c2a-4943-9ab3-363a316e9260",machine_id="857143c2dbea4a179223627cf9f47d06",mode="app_direct_mode",system_uuid="03a75ec7-5105-421a-8b8a-3d7190f6e890"} 0
machine_nvm_capacity{boot_id="a1b49bdb-4c2a-4943-9ab3-363a316e9260",machine_id="857143c2dbea4a179223627cf9f47d06",mode="memory_mode",system_uuid="03a75ec7-5105-421a-8b8a-3d7190f6e890"} 0
# HELP machine_scrape_error 1 if there was an error while getting machine metrics, 0 otherwise.
# TYPE machine_scrape_error gauge
machine_scrape_error 0
`
		benchmarkAreIdenticalSeriesFast(b, s, s+"\nfoo 1", false)
	})
	b.Run("identical-series-with-timestamps", func(b *testing.B) {
		s := `
container_ulimits_soft{container="",id="/kubelet/kubepods/burstable/pod48ea6dbad93797db01928fb7884b8154/49d928b5e3e3398730c9ce9de02171bb139b5bf2f485b153d9a293114a5762a3",image="sha256:0184c1613d92931126feb4c548e5da11015513b9e4c104e7305ee8b53b50a9da",name="49d928b5e3e3398730c9ce9de02171bb139b5bf2f485b153d9a293114a5762a3",namespace="kube-system",pod="kube-apiserver-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113856793
container_ulimits_soft{container="",id="/kubelet/kubepods/burstable/pod69cd289b4ed80ced4f95a59ff60fa102/602a9be3cad5ca8aa57bdbb4a947ddd3b1b229b6e54c7acbb6906de061d51d05",image="sha256:0184c1613d92931126feb4c548e5da11015513b9e4c104e7305ee8b53b50a9da",name="602a9be3cad5ca8aa57bdbb4a947ddd3b1b229b6e54c7acbb6906de061d51d05",namespace="kube-system",pod="kube-scheduler-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113855488
container_ulimits_soft{container="",id="/kubelet/kubepods/burstable/pod86744a0c8ef8da0d937493e4ed918cda/2f1a3706328f86337864f7c2c7100aabf9cabf03fef5518e883380977372d53f",image="sha256:0184c1613d92931126feb4c548e5da11015513b9e4c104e7305ee8b53b50a9da",name="2f1a3706328f86337864f7c2c7100aabf9cabf03fef5518e883380977372d53f",namespace="kube-system",pod="kube-controller-manager-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113858430
container_ulimits_soft{container="",id="/kubelet/kubepods/burstable/poda4a6a8d4c9c0100deb8dc3a1d3adfa32/a84ce063fb5cab82bb938151e9fa1e98ad875c3cf5dad88d797d4c65c6229c13",image="sha256:0184c1613d92931126feb4c548e5da11015513b9e4c104e7305ee8b53b50a9da",name="a84ce063fb5cab82bb938151e9fa1e98ad875c3cf5dad88d797d4c65c6229c13",namespace="kube-system",pod="etcd-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113850216
container_ulimits_soft{container="",id="/kubelet/kubepods/poda922c399-764c-4614-8a2d-84bdd6765ffc/ec6b156815cc77c389fe08a4be82603514c8929a9827b8ba27f9cb9c0b57b067",image="sha256:0184c1613d92931126feb4c548e5da11015513b9e4c104e7305ee8b53b50a9da",name="ec6b156815cc77c389fe08a4be82603514c8929a9827b8ba27f9cb9c0b57b067",namespace="kube-system",pod="kindnet-nj4p9",ulimit="max_open_files"} 1.048576e+06 1631113865193
container_ulimits_soft{container="etcd",id="/docker/6b7c234cfe92a0924e54e2a51d9607a5893a38ed14c7161f324863eeaa2fb985/kubelet/kubepods/burstable/poda4a6a8d4c9c0100deb8dc3a1d3adfa32/0cd86529af0ca0e389ed657b2c0a20f03275cf6d9e0cd52fe4c1f90b96037de7",image="k8s.gcr.io/etcd:3.4.13-0",name="0cd86529af0ca0e389ed657b2c0a20f03275cf6d9e0cd52fe4c1f90b96037de7",namespace="kube-system",pod="etcd-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113855044
container_ulimits_soft{container="etcd",id="/kubelet/kubepods/burstable/poda4a6a8d4c9c0100deb8dc3a1d3adfa32/0cd86529af0ca0e389ed657b2c0a20f03275cf6d9e0cd52fe4c1f90b96037de7",image="k8s.gcr.io/etcd:3.4.13-0",name="0cd86529af0ca0e389ed657b2c0a20f03275cf6d9e0cd52fe4c1f90b96037de7",namespace="kube-system",pod="etcd-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113867411
container_ulimits_soft{container="kindnet-cni",id="/docker/6b7c234cfe92a0924e54e2a51d9607a5893a38ed14c7161f324863eeaa2fb985/kubelet/kubepods/poda922c399-764c-4614-8a2d-84bdd6765ffc/b38094619c14a9f921e2d10fb0f84433bea774aeb223ba19dade527e1c46de22",image="docker.io/kindest/kindnetd:v20210119-d5ef916d",name="b38094619c14a9f921e2d10fb0f84433bea774aeb223ba19dade527e1c46de22",namespace="kube-system",pod="kindnet-nj4p9",ulimit="max_open_files"} 1.048576e+06 1631113868404
container_ulimits_soft{container="kindnet-cni",id="/kubelet/kubepods/poda922c399-764c-4614-8a2d-84bdd6765ffc/b38094619c14a9f921e2d10fb0f84433bea774aeb223ba19dade527e1c46de22",image="docker.io/kindest/kindnetd:v20210119-d5ef916d",name="b38094619c14a9f921e2d10fb0f84433bea774aeb223ba19dade527e1c46de22",namespace="kube-system",pod="kindnet-nj4p9",ulimit="max_open_files"} 1.048576e+06 1631113862176
container_ulimits_soft{container="kube-apiserver",id="/docker/6b7c234cfe92a0924e54e2a51d9607a5893a38ed14c7161f324863eeaa2fb985/kubelet/kubepods/burstable/pod48ea6dbad93797db01928fb7884b8154/4026cf5500d96c6e274a2607b507891abc21f7b1577e29c9400cfb0f0ce5d8aa",image="k8s.gcr.io/kube-apiserver:v1.20.2",name="4026cf5500d96c6e274a2607b507891abc21f7b1577e29c9400cfb0f0ce5d8aa",namespace="kube-system",pod="kube-apiserver-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113865919
container_ulimits_soft{container="kube-apiserver",id="/kubelet/kubepods/burstable/pod48ea6dbad93797db01928fb7884b8154/4026cf5500d96c6e274a2607b507891abc21f7b1577e29c9400cfb0f0ce5d8aa",image="k8s.gcr.io/kube-apiserver:v1.20.2",name="4026cf5500d96c6e274a2607b507891abc21f7b1577e29c9400cfb0f0ce5d8aa",namespace="kube-system",pod="kube-apiserver-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113863531
container_ulimits_soft{container="kube-controller-manager",id="/docker/6b7c234cfe92a0924e54e2a51d9607a5893a38ed14c7161f324863eeaa2fb985/kubelet/kubepods/burstable/pod86744a0c8ef8da0d937493e4ed918cda/04b0948ab58f83013fed7611f0ffadb13ff7336561c91606644848f60405771b",image="k8s.gcr.io/kube-controller-manager:v1.20.2",name="04b0948ab58f83013fed7611f0ffadb13ff7336561c91606644848f60405771b",namespace="kube-system",pod="kube-controller-manager-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113868172
container_ulimits_soft{container="kube-controller-manager",id="/kubelet/kubepods/burstable/pod86744a0c8ef8da0d937493e4ed918cda/04b0948ab58f83013fed7611f0ffadb13ff7336561c91606644848f60405771b",image="k8s.gcr.io/kube-controller-manager:v1.20.2",name="04b0948ab58f83013fed7611f0ffadb13ff7336561c91606644848f60405771b",namespace="kube-system",pod="kube-controller-manager-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113860485
container_ulimits_soft{container="kube-scheduler",id="/docker/6b7c234cfe92a0924e54e2a51d9607a5893a38ed14c7161f324863eeaa2fb985/kubelet/kubepods/burstable/pod69cd289b4ed80ced4f95a59ff60fa102/d9627625c8d60d859f2a13f9ed66c77c9767368e18eb5669fe1a85d600e43f9b",image="k8s.gcr.io/kube-scheduler:v1.20.2",name="d9627625c8d60d859f2a13f9ed66c77c9767368e18eb5669fe1a85d600e43f9b",namespace="kube-system",pod="kube-scheduler-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113857794
container_ulimits_soft{container="kube-scheduler",id="/kubelet/kubepods/burstable/pod69cd289b4ed80ced4f95a59ff60fa102/d9627625c8d60d859f2a13f9ed66c77c9767368e18eb5669fe1a85d600e43f9b",image="k8s.gcr.io/kube-scheduler:v1.20.2",name="d9627625c8d60d859f2a13f9ed66c77c9767368e18eb5669fe1a85d600e43f9b",namespace="kube-system",pod="kube-scheduler-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113868640
`
		benchmarkAreIdenticalSeriesFast(b, s, s, true)
	})
	b.Run("different-series-with-timestamps", func(b *testing.B) {
		s := `
container_ulimits_soft{container="",id="/kubelet/kubepods/burstable/pod48ea6dbad93797db01928fb7884b8154/49d928b5e3e3398730c9ce9de02171bb139b5bf2f485b153d9a293114a5762a3",image="sha256:0184c1613d92931126feb4c548e5da11015513b9e4c104e7305ee8b53b50a9da",name="49d928b5e3e3398730c9ce9de02171bb139b5bf2f485b153d9a293114a5762a3",namespace="kube-system",pod="kube-apiserver-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113856793
container_ulimits_soft{container="",id="/kubelet/kubepods/burstable/pod69cd289b4ed80ced4f95a59ff60fa102/602a9be3cad5ca8aa57bdbb4a947ddd3b1b229b6e54c7acbb6906de061d51d05",image="sha256:0184c1613d92931126feb4c548e5da11015513b9e4c104e7305ee8b53b50a9da",name="602a9be3cad5ca8aa57bdbb4a947ddd3b1b229b6e54c7acbb6906de061d51d05",namespace="kube-system",pod="kube-scheduler-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113855488
container_ulimits_soft{container="",id="/kubelet/kubepods/burstable/pod86744a0c8ef8da0d937493e4ed918cda/2f1a3706328f86337864f7c2c7100aabf9cabf03fef5518e883380977372d53f",image="sha256:0184c1613d92931126feb4c548e5da11015513b9e4c104e7305ee8b53b50a9da",name="2f1a3706328f86337864f7c2c7100aabf9cabf03fef5518e883380977372d53f",namespace="kube-system",pod="kube-controller-manager-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113858430
container_ulimits_soft{container="",id="/kubelet/kubepods/burstable/poda4a6a8d4c9c0100deb8dc3a1d3adfa32/a84ce063fb5cab82bb938151e9fa1e98ad875c3cf5dad88d797d4c65c6229c13",image="sha256:0184c1613d92931126feb4c548e5da11015513b9e4c104e7305ee8b53b50a9da",name="a84ce063fb5cab82bb938151e9fa1e98ad875c3cf5dad88d797d4c65c6229c13",namespace="kube-system",pod="etcd-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113850216
container_ulimits_soft{container="",id="/kubelet/kubepods/poda922c399-764c-4614-8a2d-84bdd6765ffc/ec6b156815cc77c389fe08a4be82603514c8929a9827b8ba27f9cb9c0b57b067",image="sha256:0184c1613d92931126feb4c548e5da11015513b9e4c104e7305ee8b53b50a9da",name="ec6b156815cc77c389fe08a4be82603514c8929a9827b8ba27f9cb9c0b57b067",namespace="kube-system",pod="kindnet-nj4p9",ulimit="max_open_files"} 1.048576e+06 1631113865193
container_ulimits_soft{container="etcd",id="/docker/6b7c234cfe92a0924e54e2a51d9607a5893a38ed14c7161f324863eeaa2fb985/kubelet/kubepods/burstable/poda4a6a8d4c9c0100deb8dc3a1d3adfa32/0cd86529af0ca0e389ed657b2c0a20f03275cf6d9e0cd52fe4c1f90b96037de7",image="k8s.gcr.io/etcd:3.4.13-0",name="0cd86529af0ca0e389ed657b2c0a20f03275cf6d9e0cd52fe4c1f90b96037de7",namespace="kube-system",pod="etcd-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113855044
container_ulimits_soft{container="etcd",id="/kubelet/kubepods/burstable/poda4a6a8d4c9c0100deb8dc3a1d3adfa32/0cd86529af0ca0e389ed657b2c0a20f03275cf6d9e0cd52fe4c1f90b96037de7",image="k8s.gcr.io/etcd:3.4.13-0",name="0cd86529af0ca0e389ed657b2c0a20f03275cf6d9e0cd52fe4c1f90b96037de7",namespace="kube-system",pod="etcd-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113867411
container_ulimits_soft{container="kindnet-cni",id="/docker/6b7c234cfe92a0924e54e2a51d9607a5893a38ed14c7161f324863eeaa2fb985/kubelet/kubepods/poda922c399-764c-4614-8a2d-84bdd6765ffc/b38094619c14a9f921e2d10fb0f84433bea774aeb223ba19dade527e1c46de22",image="docker.io/kindest/kindnetd:v20210119-d5ef916d",name="b38094619c14a9f921e2d10fb0f84433bea774aeb223ba19dade527e1c46de22",namespace="kube-system",pod="kindnet-nj4p9",ulimit="max_open_files"} 1.048576e+06 1631113868404
container_ulimits_soft{container="kindnet-cni",id="/kubelet/kubepods/poda922c399-764c-4614-8a2d-84bdd6765ffc/b38094619c14a9f921e2d10fb0f84433bea774aeb223ba19dade527e1c46de22",image="docker.io/kindest/kindnetd:v20210119-d5ef916d",name="b38094619c14a9f921e2d10fb0f84433bea774aeb223ba19dade527e1c46de22",namespace="kube-system",pod="kindnet-nj4p9",ulimit="max_open_files"} 1.048576e+06 1631113862176
container_ulimits_soft{container="kube-apiserver",id="/docker/6b7c234cfe92a0924e54e2a51d9607a5893a38ed14c7161f324863eeaa2fb985/kubelet/kubepods/burstable/pod48ea6dbad93797db01928fb7884b8154/4026cf5500d96c6e274a2607b507891abc21f7b1577e29c9400cfb0f0ce5d8aa",image="k8s.gcr.io/kube-apiserver:v1.20.2",name="4026cf5500d96c6e274a2607b507891abc21f7b1577e29c9400cfb0f0ce5d8aa",namespace="kube-system",pod="kube-apiserver-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113865919
container_ulimits_soft{container="kube-apiserver",id="/kubelet/kubepods/burstable/pod48ea6dbad93797db01928fb7884b8154/4026cf5500d96c6e274a2607b507891abc21f7b1577e29c9400cfb0f0ce5d8aa",image="k8s.gcr.io/kube-apiserver:v1.20.2",name="4026cf5500d96c6e274a2607b507891abc21f7b1577e29c9400cfb0f0ce5d8aa",namespace="kube-system",pod="kube-apiserver-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113863531
container_ulimits_soft{container="kube-controller-manager",id="/docker/6b7c234cfe92a0924e54e2a51d9607a5893a38ed14c7161f324863eeaa2fb985/kubelet/kubepods/burstable/pod86744a0c8ef8da0d937493e4ed918cda/04b0948ab58f83013fed7611f0ffadb13ff7336561c91606644848f60405771b",image="k8s.gcr.io/kube-controller-manager:v1.20.2",name="04b0948ab58f83013fed7611f0ffadb13ff7336561c91606644848f60405771b",namespace="kube-system",pod="kube-controller-manager-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113868172
container_ulimits_soft{container="kube-controller-manager",id="/kubelet/kubepods/burstable/pod86744a0c8ef8da0d937493e4ed918cda/04b0948ab58f83013fed7611f0ffadb13ff7336561c91606644848f60405771b",image="k8s.gcr.io/kube-controller-manager:v1.20.2",name="04b0948ab58f83013fed7611f0ffadb13ff7336561c91606644848f60405771b",namespace="kube-system",pod="kube-controller-manager-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113860485
container_ulimits_soft{container="kube-scheduler",id="/docker/6b7c234cfe92a0924e54e2a51d9607a5893a38ed14c7161f324863eeaa2fb985/kubelet/kubepods/burstable/pod69cd289b4ed80ced4f95a59ff60fa102/d9627625c8d60d859f2a13f9ed66c77c9767368e18eb5669fe1a85d600e43f9b",image="k8s.gcr.io/kube-scheduler:v1.20.2",name="d9627625c8d60d859f2a13f9ed66c77c9767368e18eb5669fe1a85d600e43f9b",namespace="kube-system",pod="kube-scheduler-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113857794
container_ulimits_soft{container="kube-scheduler",id="/kubelet/kubepods/burstable/pod69cd289b4ed80ced4f95a59ff60fa102/d9627625c8d60d859f2a13f9ed66c77c9767368e18eb5669fe1a85d600e43f9b",image="k8s.gcr.io/kube-scheduler:v1.20.2",name="d9627625c8d60d859f2a13f9ed66c77c9767368e18eb5669fe1a85d600e43f9b",namespace="kube-system",pod="kube-scheduler-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113868640
`
		benchmarkAreIdenticalSeriesFast(b, s, s+"\nfoo 1", false)
	})
}

func benchmarkAreIdenticalSeriesFast(b *testing.B, s1, s2 string, expectedResult bool) {
	b.SetBytes(int64(len(s1)))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			result := AreIdenticalSeriesFast(s1, s2)
			if result != expectedResult {
				panic(fmt.Errorf("unexpected result; got %v; want %v", result, expectedResult))
			}
		}
	})
}

func BenchmarkRowsDiff(b *testing.B) {
	s1 := `container_ulimits_soft{container="",id="/kubelet/kubepods/burstable/pod48ea6dbad93797db01928fb7884b8154/49d928b5e3e3398730c9ce9de02171bb139b5bf2f485b153d9a293114a5762a3",image="sha256:0184c1613d92931126feb4c548e5da11015513b9e4c104e7305ee8b53b50a9da",name="49d928b5e3e3398730c9ce9de02171bb139b5bf2f485b153d9a293114a5762a3",namespace="kube-system",pod="kube-apiserver-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113856793
container_ulimits_soft{container="",id="/kubelet/kubepods/burstable/pod69cd289b4ed80ced4f95a59ff60fa102/602a9be3cad5ca8aa57bdbb4a947ddd3b1b229b6e54c7acbb6906de061d51d05",image="sha256:0184c1613d92931126feb4c548e5da11015513b9e4c104e7305ee8b53b50a9da",name="602a9be3cad5ca8aa57bdbb4a947ddd3b1b229b6e54c7acbb6906de061d51d05",namespace="kube-system",pod="kube-scheduler-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113855488
container_ulimits_soft{container="",id="/kubelet/kubepods/burstable/pod86744a0c8ef8da0d937493e4ed918cda/2f1a3706328f86337864f7c2c7100aabf9cabf03fef5518e883380977372d53f",image="sha256:0184c1613d92931126feb4c548e5da11015513b9e4c104e7305ee8b53b50a9da",name="2f1a3706328f86337864f7c2c7100aabf9cabf03fef5518e883380977372d53f",namespace="kube-system",pod="kube-controller-manager-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113858430
container_ulimits_soft{container="",id="/kubelet/kubepods/burstable/poda4a6a8d4c9c0100deb8dc3a1d3adfa32/a84ce063fb5cab82bb938151e9fa1e98ad875c3cf5dad88d797d4c65c6229c13",image="sha256:0184c1613d92931126feb4c548e5da11015513b9e4c104e7305ee8b53b50a9da",name="a84ce063fb5cab82bb938151e9fa1e98ad875c3cf5dad88d797d4c65c6229c13",namespace="kube-system",pod="etcd-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113850216
container_ulimits_soft{container="",id="/kubelet/kubepods/poda922c399-764c-4614-8a2d-84bdd6765ffc/ec6b156815cc77c389fe08a4be82603514c8929a9827b8ba27f9cb9c0b57b067",image="sha256:0184c1613d92931126feb4c548e5da11015513b9e4c104e7305ee8b53b50a9da",name="ec6b156815cc77c389fe08a4be82603514c8929a9827b8ba27f9cb9c0b57b067",namespace="kube-system",pod="kindnet-nj4p9",ulimit="max_open_files"} 1.048576e+06 1631113865193
container_ulimits_soft{container="etcd",id="/docker/6b7c234cfe92a0924e54e2a51d9607a5893a38ed14c7161f324863eeaa2fb985/kubelet/kubepods/burstable/poda4a6a8d4c9c0100deb8dc3a1d3adfa32/0cd86529af0ca0e389ed657b2c0a20f03275cf6d9e0cd52fe4c1f90b96037de7",image="k8s.gcr.io/etcd:3.4.13-0",name="0cd86529af0ca0e389ed657b2c0a20f03275cf6d9e0cd52fe4c1f90b96037de7",namespace="kube-system",pod="etcd-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113855044
container_ulimits_soft{container="etcd",id="/kubelet/kubepods/burstable/poda4a6a8d4c9c0100deb8dc3a1d3adfa32/0cd86529af0ca0e389ed657b2c0a20f03275cf6d9e0cd52fe4c1f90b96037de7",image="k8s.gcr.io/etcd:3.4.13-0",name="0cd86529af0ca0e389ed657b2c0a20f03275cf6d9e0cd52fe4c1f90b96037de7",namespace="kube-system",pod="etcd-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113867411
container_ulimits_soft{container="kindnet-cni",id="/docker/6b7c234cfe92a0924e54e2a51d9607a5893a38ed14c7161f324863eeaa2fb985/kubelet/kubepods/poda922c399-764c-4614-8a2d-84bdd6765ffc/b38094619c14a9f921e2d10fb0f84433bea774aeb223ba19dade527e1c46de22",image="docker.io/kindest/kindnetd:v20210119-d5ef916d",name="b38094619c14a9f921e2d10fb0f84433bea774aeb223ba19dade527e1c46de22",namespace="kube-system",pod="kindnet-nj4p9",ulimit="max_open_files"} 1.048576e+06 1631113868404
container_ulimits_soft{container="kindnet-cni",id="/kubelet/kubepods/poda922c399-764c-4614-8a2d-84bdd6765ffc/b38094619c14a9f921e2d10fb0f84433bea774aeb223ba19dade527e1c46de22",image="docker.io/kindest/kindnetd:v20210119-d5ef916d",name="b38094619c14a9f921e2d10fb0f84433bea774aeb223ba19dade527e1c46de22",namespace="kube-system",pod="kindnet-nj4p9",ulimit="max_open_files"} 1.048576e+06 1631113862176
container_ulimits_soft{container="kube-apiserver",id="/docker/6b7c234cfe92a0924e54e2a51d9607a5893a38ed14c7161f324863eeaa2fb985/kubelet/kubepods/burstable/pod48ea6dbad93797db01928fb7884b8154/4026cf5500d96c6e274a2607b507891abc21f7b1577e29c9400cfb0f0ce5d8aa",image="k8s.gcr.io/kube-apiserver:v1.20.2",name="4026cf5500d96c6e274a2607b507891abc21f7b1577e29c9400cfb0f0ce5d8aa",namespace="kube-system",pod="kube-apiserver-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113865919
container_ulimits_soft{container="kube-apiserver",id="/kubelet/kubepods/burstable/pod48ea6dbad93797db01928fb7884b8154/4026cf5500d96c6e274a2607b507891abc21f7b1577e29c9400cfb0f0ce5d8aa",image="k8s.gcr.io/kube-apiserver:v1.20.2",name="4026cf5500d96c6e274a2607b507891abc21f7b1577e29c9400cfb0f0ce5d8aa",namespace="kube-system",pod="kube-apiserver-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113863531
container_ulimits_soft{container="kube-controller-manager",id="/docker/6b7c234cfe92a0924e54e2a51d9607a5893a38ed14c7161f324863eeaa2fb985/kubelet/kubepods/burstable/pod86744a0c8ef8da0d937493e4ed918cda/04b0948ab58f83013fed7611f0ffadb13ff7336561c91606644848f60405771b",image="k8s.gcr.io/kube-controller-manager:v1.20.2",name="04b0948ab58f83013fed7611f0ffadb13ff7336561c91606644848f60405771b",namespace="kube-system",pod="kube-controller-manager-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113868172
container_ulimits_soft{container="kube-controller-manager",id="/kubelet/kubepods/burstable/pod86744a0c8ef8da0d937493e4ed918cda/04b0948ab58f83013fed7611f0ffadb13ff7336561c91606644848f60405771b",image="k8s.gcr.io/kube-controller-manager:v1.20.2",name="04b0948ab58f83013fed7611f0ffadb13ff7336561c91606644848f60405771b",namespace="kube-system",pod="kube-controller-manager-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113860485
container_ulimits_soft{container="kube-scheduler",id="/docker/6b7c234cfe92a0924e54e2a51d9607a5893a38ed14c7161f324863eeaa2fb985/kubelet/kubepods/burstable/pod69cd289b4ed80ced4f95a59ff60fa102/d9627625c8d60d859f2a13f9ed66c77c9767368e18eb5669fe1a85d600e43f9b",image="k8s.gcr.io/kube-scheduler:v1.20.2",name="d9627625c8d60d859f2a13f9ed66c77c9767368e18eb5669fe1a85d600e43f9b",namespace="kube-system",pod="kube-scheduler-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113857794
container_ulimits_soft{container="kube-scheduler",id="/kubelet/kubepods/burstable/pod69cd289b4ed80ced4f95a59ff60fa102/d9627625c8d60d859f2a13f9ed66c77c9767368e18eb5669fe1a85d600e43f9b",image="k8s.gcr.io/kube-scheduler:v1.20.2",name="d9627625c8d60d859f2a13f9ed66c77c9767368e18eb5669fe1a85d600e43f9b",namespace="kube-system",pod="kube-scheduler-kind-control-plane",ulimit="max_open_files"} 1.048576e+06 1631113868640
`
	s2 := s1 + "\nfoo 123"
	b.SetBytes(int64(len(s1)))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			diff := GetRowsDiff(s2, s1)
			if diff != "foo 0\n" {
				panic(fmt.Errorf("unexpected diff; got %q; want %q", diff, "foo 0\n"))
			}
		}
	})
}

func BenchmarkRowsUnmarshal(b *testing.B) {
	s := `cpu_usage{mode="user"} 1.23
cpu_usage{mode="system"} 23.344
cpu_usage{mode="iowait"} 3.3443
cpu_usage{mode="irq"} 0.34432
`
	b.SetBytes(int64(len(s)))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var rows Rows
		for pb.Next() {
			rows.Unmarshal(s)
			if len(rows.Rows) != 4 {
				panic(fmt.Errorf("unexpected number of rows unmarshaled: got %d; want 4", len(rows.Rows)))
			}
		}
	})
}
