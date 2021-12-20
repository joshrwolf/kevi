module cattle.io/kevi

go 1.16

require (
	github.com/argoproj/gitops-engine v0.5.1
	github.com/fluxcd/pkg/ssa v0.7.0
	github.com/go-logr/logr v1.2.0
	github.com/google/go-containerregistry v0.7.0
	github.com/mholt/archiver/v3 v3.5.1
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/open-policy-agent/cert-controller v0.2.0
	github.com/opencontainers/image-spec v1.0.2
	github.com/rancherfederal/ocil v0.1.4
	github.com/rs/zerolog v1.26.1
	github.com/spf13/cobra v1.2.1
	helm.sh/helm/v3 v3.6.1-0.20211207164812-8ca401398d8b
	k8s.io/api v0.23.0
	k8s.io/apimachinery v0.23.0
	k8s.io/client-go v0.23.0
	oras.land/oras-go v1.0.0
	sigs.k8s.io/cli-utils v0.26.1
	sigs.k8s.io/controller-runtime v0.11.0
	sigs.k8s.io/kustomize/api v0.10.1
	sigs.k8s.io/kustomize/kyaml v0.13.0
	sigs.k8s.io/yaml v1.3.0
)

replace (
	//github.com/go-logr/logr v1.2.0 => github.com/go-logr/logr v0.4.0
	// https://github.com/kubernetes/kubernetes/issues/79384#issuecomment-505627280
	k8s.io/api => k8s.io/api v0.22.2
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.22.2 // indirect
	k8s.io/apimachinery => k8s.io/apimachinery v0.22.2 // indirect
	k8s.io/apiserver => k8s.io/apiserver v0.22.2
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.22.2
	k8s.io/client-go => k8s.io/client-go v0.22.2
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.22.2
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.22.2
	k8s.io/code-generator => k8s.io/code-generator v0.22.2
	k8s.io/component-base => k8s.io/component-base v0.22.2
	k8s.io/component-helpers => k8s.io/component-helpers v0.22.2
	k8s.io/controller-manager => k8s.io/controller-manager v0.22.2
	k8s.io/cri-api => k8s.io/cri-api v0.22.2
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.22.2
	//k8s.io/klog/v2 v2.30.0 => k8s.io/klog/v2 v2.9.0
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.22.2
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.22.2
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.22.2
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.22.2
	k8s.io/kubectl => k8s.io/kubectl v0.22.2
	k8s.io/kubelet => k8s.io/kubelet v0.22.2
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.22.2
	k8s.io/metrics => k8s.io/metrics v0.22.2
	k8s.io/mount-utils => k8s.io/mount-utils v0.22.2
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.22.2
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.22.2
)
