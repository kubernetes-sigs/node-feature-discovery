module sigs.k8s.io/node-feature-discovery

go 1.13

require (
	github.com/docopt/docopt-go v0.0.0-20180111231733-ee0de3bc6815
	github.com/golang/protobuf v1.3.2
	github.com/klauspost/cpuid v1.2.3
	github.com/onsi/ginkgo v1.10.1
	github.com/onsi/gomega v1.7.0
	github.com/smartystreets/goconvey v0.0.0-20190330032615-68dc04aab96a
	github.com/stretchr/testify v1.4.0
	github.com/vektra/errors v0.0.0-20140903201135-c64d83aba85a
	golang.org/x/net v0.0.0-20191004110552-13f9640d40b9
	google.golang.org/grpc v1.23.1
	k8s.io/api v0.17.2
	k8s.io/apimachinery v0.17.2
	k8s.io/client-go v0.17.2
	k8s.io/component-base v0.17.2
	k8s.io/klog v1.0.0
	k8s.io/kubernetes v1.17.2
	k8s.io/utils v0.0.0-20191114184206-e782cd3c129f
	sigs.k8s.io/yaml v1.1.0
)

// The k8s "sub-"packages do not have 'semver' compatible versions. Thus, we
// need to override with commits (corresponding their kubernetes-* tags)
replace (
	k8s.io/api => k8s.io/api v0.0.0-20200121193204-7ea599edc7fd
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20200121201129-111e9ba415da
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20191121175448-79c2a76c473a
	k8s.io/apiserver => k8s.io/apiserver v0.0.0-20200121195158-da2f3bd69287
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20200121201805-7928b415bdea
	k8s.io/client-go => k8s.io/client-go v0.0.0-20200121193945-bdedab45d4f6
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.0.0-20200121203829-580c13bb6ed9
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.0.0-20200121203528-48c15d793bf4
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20191121175249-e95606b614f0
	k8s.io/component-base => k8s.io/component-base v0.0.0-20200121194253-47d744dd27ec
	k8s.io/cri-api => k8s.io/cri-api v0.0.0-20191121183020-775aa3c1cf73
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.0.0-20200121204128-ab1d1be7e7e9
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20200121195706-c8017da6deb7
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.0.0-20200121203241-7fc8a284e25f
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.0.0-20200121202405-597cb7b43db3
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.0.0-20200121202948-05dd8b0a4787
	k8s.io/kubectl => k8s.io/kubectl v0.0.0-20200121205541-a36079a4286a
	k8s.io/kubelet => k8s.io/kubelet v0.0.0-20200121202654-3d0d0a3a4b44
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.0.0-20200121204546-147d309c2148
	k8s.io/metrics => k8s.io/metrics v0.0.0-20200121201502-3a7afb0af1bc
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.0.0-20200121200150-07ea3fc70559
)
