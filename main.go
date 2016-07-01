package main

import (
	"log"
	"os"

	"github.com/klauspost/cpuid"
	client "k8s.io/kubernetes/pkg/client/unversioned"
)

func main() {
	//Setting-up K8S client
	cli, err := client.NewInCluster()
	if err != nil {
		log.Fatalf("Can't Get K8s Client:%v", err)
	}

	features := cpuid.CPU.Features.Strings()

	podName := os.Getenv("POD_NAME")
	podns := os.Getenv("POD_NAMESPACE")

	pod, err := cli.Pods(podns).Get(podName)
	if err != nil {
		log.Fatalf("Can't Get Pod:%v", err)
	}

	node, err := cli.Nodes().Get(pod.Spec.NodeName)
	if err != nil {
		log.Fatalf("Can't Get Node:%v", err)
	}

	for _, feature := range features {
		node.Labels["node.alpha.intel.com/"+feature] = "true"
	}

	_, err = cli.Nodes().Update(node)
	if err != nil {
		log.Fatalf("Can't Update Node:%v", err)
	}
}
