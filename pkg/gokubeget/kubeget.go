package gokubeget

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

// KubeGet provides a kubectl get-like interface for fetching Kubernetes resources
type KubeGet struct {
	restMapper      meta.RESTMapper
	dynamicClient   dynamic.Interface
	discoveryClient discovery.CachedDiscoveryInterface
}

// NewKubeGet creates a new KubeGet instance using the provided Kubernetes configuration
func NewKubeGet(config *rest.Config) (*KubeGet, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}

	// Wrap discovery client with memory cache for efficient resource discovery
	cachedClient := memory.NewMemCacheClient(discoveryClient)

	restMapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedClient)

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &KubeGet{
		restMapper:      restMapper,
		dynamicClient:   dynamicClient,
		discoveryClient: cachedClient,
	}, nil
}

// Get retrieves Kubernetes resources by name and namespace, returning the resolved GVR and resource list
// If namespace is empty, retrieves cluster-scoped resources
func (k *KubeGet) Get(ctx context.Context, resourceName, namespace string) (schema.GroupVersionResource, *unstructured.UnstructuredList, error) {
	gvr, err := k.findGVR(resourceName)
	if err != nil {
		return schema.GroupVersionResource{}, nil, fmt.Errorf("failed to find resource %q: %w", resourceName, err)
	}

	var resourceInterface dynamic.ResourceInterface
	if namespace != "" {
		resourceInterface = k.dynamicClient.Resource(gvr).Namespace(namespace)
	} else {
		resourceInterface = k.dynamicClient.Resource(gvr)
	}

	list, err := resourceInterface.List(ctx, metav1.ListOptions{})
	if err != nil {
		return gvr, nil, fmt.Errorf("failed to list resources: %w", err)
	}

	return gvr, list, nil
}

// findGVR resolves a resource name (kind, plural, or shortname) to its GroupVersionResource
func (k *KubeGet) findGVR(resourceName string) (schema.GroupVersionResource, error) {
	if resourceName == "" {
		return schema.GroupVersionResource{}, fmt.Errorf("resource name cannot be empty")
	}
	// Handle fully qualified resource names like "datasciencepipelinesapplications.v1.datasciencepipelinesapplications.opendatahub.io"
	if strings.Contains(resourceName, ".") {
		parts := strings.Split(resourceName, ".")
		if len(parts) >= 3 {
			// Format: resource.version.group (may have multiple dots in group)
			resourceOnly := parts[0]
			version := parts[1]
			group := strings.Join(parts[2:], ".")

			// Construct the GroupVersionResource directly
			return schema.GroupVersionResource{
				Group:    group,
				Version:  version,
				Resource: resourceOnly,
			}, nil
		}
	}

	// First try to find it as a resource name (plural form) - this handles most cases
	gvr, err := k.restMapper.ResourceFor(schema.GroupVersionResource{
		Resource: resourceName,
	})
	if err == nil {
		return gvr, nil
	}

	// Try to find by kind name (case-insensitive search across all groups)
	mappings, err := k.restMapper.RESTMappings(schema.GroupKind{Kind: resourceName})
	if err == nil && len(mappings) > 0 {
		return mappings[0].Resource, nil
	}

	// Try case variations for kind names (e.g., "dspa" -> "DSPA")
	kindVariations := []string{
		resourceName,
		strings.Title(resourceName),
		strings.ToUpper(resourceName),
		strings.ToUpper(string(resourceName[0])) + strings.ToLower(resourceName[1:]),
	}

	for _, kind := range kindVariations {
		mappings, err := k.restMapper.RESTMappings(schema.GroupKind{Kind: kind})
		if err == nil && len(mappings) > 0 {
			return mappings[0].Resource, nil
		}
	}

	// Last resort: try to find by resource shortnames or aliases
	// This requires checking all available resources
	apiResourceLists, err := k.discoveryClient.ServerPreferredResources()
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("failed to find resource %q: %w", resourceName, err)
	}

	for _, apiResourceList := range apiResourceLists {
		if apiResourceList == nil {
			continue
		}

		gv, err := schema.ParseGroupVersion(apiResourceList.GroupVersion)
		if err != nil {
			continue
		}

		for _, apiResource := range apiResourceList.APIResources {
			// Check if the resourceName matches the resource name, kind, or any shortnames
			if apiResource.Name == resourceName ||
				strings.EqualFold(apiResource.Kind, resourceName) {
				return gv.WithResource(apiResource.Name), nil
			}

			for _, shortName := range apiResource.ShortNames {
				if shortName == resourceName {
					return gv.WithResource(apiResource.Name), nil
				}
			}
		}
	}

	return schema.GroupVersionResource{}, fmt.Errorf("failed to find resource %q: resource not found in any API group", resourceName)
}
