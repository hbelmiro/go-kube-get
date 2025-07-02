package main

import (
	"context"
	"fmt"
	"os"

	"go-kube-get/pkg/gokubeget"
	"path/filepath"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <resource-name> [namespace]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s pods default\n", os.Args[0])
		os.Exit(1)
	}

	resourceName := os.Args[1]
	var namespace string
	if len(os.Args) > 2 {
		namespace = os.Args[2]
	}

	kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
	if envKubeconfig := os.Getenv("KUBECONFIG"); envKubeconfig != "" {
		kubeconfig = envKubeconfig
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load kubeconfig: %v\n", err)
		os.Exit(1)
	}

	// Load the current namespace from kubeconfig
	clientConfig, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load kubeconfig file: %v\n", err)
		os.Exit(1)
	}

	currentContext := clientConfig.CurrentContext
	defaultNamespace := "default"
	if context, exists := clientConfig.Contexts[currentContext]; exists && context.Namespace != "" {
		defaultNamespace = context.Namespace
	}

	kubeget, err := gokubeget.NewKubeGet(config, defaultNamespace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create kubeget client: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	gvr, resourceList, err := kubeget.Get(ctx, resourceName, namespace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get resources: %v\n", err)
		os.Exit(1)
	}

	// Show which namespace was actually used
	actualNamespace := namespace
	if actualNamespace == "" {
		actualNamespace = defaultNamespace
	}

	fmt.Printf("Resource: %s (Group: %s, Version: %s, Resource: %s)\n",
		resourceName, gvr.Group, gvr.Version, gvr.Resource)
	fmt.Printf("Namespace: %s\n\n", actualNamespace)

	if len(resourceList.Items) == 0 {
		fmt.Println("No resources found.")
		return
	}

	fmt.Printf("Found %d resource(s):\n", len(resourceList.Items))
	for i, item := range resourceList.Items {
		fmt.Printf("%d. %s\n", i+1, item.GetName())
	}
}
