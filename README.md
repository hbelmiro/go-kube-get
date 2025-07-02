# go-kube-get

A `kubectl`-compatible Go library for fetching Kubernetes resources.

## Why not just use the standard Kubernetes Go client?

**TL;DR: This library lets users specify resources like they do with `kubectl` - no need to map `sa` to `serviceaccounts.v1.core`.**

With [client-go](https://github.com/kubernetes/client-go), you need to know exact API groups and versions for each resource. This library handles that mapping automatically, just like `kubectl` does.

### The Problem

```go
// With client-go, users can't just say "deploy" - you need this mapping:
func getResource(userInput string) (interface{}, error) {
    switch userInput {
    case "pods", "pod", "po":
        return clientset.CoreV1().Pods("ns").List(ctx, metav1.ListOptions{})
    case "deployments", "deployment", "deploy":
        return clientset.AppsV1().Deployments("ns").List(ctx, metav1.ListOptions{})
    // ... hundreds more cases
    }
}
```

### The Solution

```go
// With go-kube-get, it just works:
kubeget, _ := gokubeget.NewKubeGet(config)
gvr, resources, err := kubeget.Get(ctx, "deploy", "default") // or "po", "svc", etc.
```

**Use go-kube-get when:** Your application accepts user input for resource names

**Use client-go when:** You know exactly which resources you need, or need complex operations (create, update, watch)

## Installation

```bash
go get github.com/hbelmiro/go-kube-get
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/hbelmiro/go-kube-get/pkg/gokubeget"
    "k8s.io/client-go/tools/clientcmd"
    "k8s.io/client-go/util/homedir"
    "path/filepath"
)

func main() {
    // Load kubeconfig
    kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
    config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
    if err != nil {
        log.Fatal(err)
    }

    // Create client
    kubeget, err := gokubeget.NewKubeGet(config)
    if err != nil {
        log.Fatal(err)
    }

    // Get resources using kubectl-style names
    ctx := context.Background()
    gvr, resources, err := kubeget.Get(ctx, "pods", "default")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Found %d pods\n", len(resources.Items))
    for _, pod := range resources.Items {
        fmt.Printf("- %s\n", pod.GetName())
    }
}
```

## API

### Constructor

```go
kubeget, err := gokubeget.NewKubeGet(config)
```

- `config`: Kubernetes client configuration

### Get Resources

```go
gvr, resources, err := kubeget.Get(ctx, resourceName, namespace)
```

- Returns: GroupVersionResource, resource list, error
- `namespace`: Target namespace (empty string for cluster-scoped resources)

## Supported Resource Names

All kubectl formats work:

```go
// Short names
kubeget.Get(ctx, "po", "default")      // pods
kubeget.Get(ctx, "svc", "default")     // services  
kubeget.Get(ctx, "deploy", "default")  // deployments

// Plural names
kubeget.Get(ctx, "pods", "default")
kubeget.Get(ctx, "services", "default")

// Kind names (case-insensitive)
kubeget.Get(ctx, "Pod", "default")
kubeget.Get(ctx, "SERVICE", "default")

// Fully qualified names
kubeget.Get(ctx, "pods.v1.", "default")
kubeget.Get(ctx, "deployments.v1.apps", "default")
kubeget.Get(ctx, "customresource.v1.example.com", "default")

// Cluster-scoped resources (empty namespace)
kubeget.Get(ctx, "nodes", "")
kubeget.Get(ctx, "clusterroles", "")
```

## Examples

```go
// List pods in specific namespace
gvr, pods, err := kubeget.Get(ctx, "pods", "default")

// List services in kube-system namespace  
gvr, services, err := kubeget.Get(ctx, "services", "kube-system")

// List cluster-scoped resources (nodes, clusterroles, etc.)
gvr, nodes, err := kubeget.Get(ctx, "nodes", "")

// Custom resources work too
gvr, resources, err := kubeget.Get(ctx, "myresource.v1.example.com", "my-namespace")

// With timeout
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
gvr, resources, err := kubeget.Get(ctx, "pods", "default")
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request
