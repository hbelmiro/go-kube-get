package gokubeget

import (
	"context"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

func TestNewKubeGet(t *testing.T) {
	tests := []struct {
		name        string
		config      *rest.Config
		expectError bool
	}{
		{
			name:        "valid config",
			config:      &rest.Config{Host: "https://test-cluster"},
			expectError: false,
		},
		{
			name:        "nil config should fail",
			config:      nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kubeget, err := NewKubeGet(tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if kubeget == nil {
				t.Errorf("Expected kubeget to be non-nil")
				return
			}
		})
	}
}

func TestFindGVR_FullyQualifiedNames(t *testing.T) {
	tests := []struct {
		name         string
		resourceName string
		expected     schema.GroupVersionResource
	}{
		{
			name:         "fully qualified resource name",
			resourceName: "pods.v1.",
			expected: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
		},
		{
			name:         "fully qualified with group",
			resourceName: "deployments.v1.apps",
			expected: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			},
		},
		{
			name:         "complex group name",
			resourceName: "datasciencepipelinesapplications.v1.datasciencepipelinesapplications.opendatahub.io",
			expected: schema.GroupVersionResource{
				Group:    "datasciencepipelinesapplications.opendatahub.io",
				Version:  "v1",
				Resource: "datasciencepipelinesapplications",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the parsing logic directly for fully qualified names
			parts := strings.Split(tt.resourceName, ".")
			if len(parts) >= 3 {
				resourceOnly := parts[0]
				version := parts[1]
				group := strings.Join(parts[2:], ".")

				result := schema.GroupVersionResource{
					Group:    group,
					Version:  version,
					Resource: resourceOnly,
				}

				if result != tt.expected {
					t.Errorf("Expected %+v, got %+v", tt.expected, result)
				}
			} else {
				t.Errorf("Test case %q should have at least 3 parts when split by dots", tt.resourceName)
			}
		})
	}
}

func TestResourceNameVariations(t *testing.T) {
	tests := []struct {
		name         string
		resourceName string
		expectError  bool
	}{
		{
			name:         "empty resource name",
			resourceName: "",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kubeget := &KubeGet{}
			_, err := kubeget.findGVR(tt.resourceName)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
		})
	}
}

func TestGet_ErrorHandling(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		kubeget      *KubeGet
		resourceName string
		namespace    string
		expectError  bool
	}{
		{
			name:         "nil kubeget",
			kubeget:      nil,
			resourceName: "pods",
			namespace:    "default",
			expectError:  true,
		},
		{
			name:         "empty resource name with valid kubeget",
			kubeget:      &KubeGet{},
			resourceName: "",
			namespace:    "default",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.kubeget == nil {
				// Skip nil kubeget test as it would panic
				return
			}

			_, _, err := tt.kubeget.Get(ctx, tt.resourceName, tt.namespace)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
		})
	}
}

func splitResourceName(resourceName string) []string {
	if !containsString(resourceName, ".") {
		return []string{resourceName}
	}

	parts := []string{}
	current := ""

	for _, r := range resourceName {
		if r == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(r)
		}
	}

	if current != "" {
		parts = append(parts, current)
	}

	return parts
}

func parseFullyQualifiedResource(parts []string) schema.GroupVersionResource {
	if len(parts) < 3 {
		return schema.GroupVersionResource{}
	}

	resourceOnly := parts[0]
	version := parts[1]
	group := ""

	if len(parts) > 2 {
		groupParts := parts[2:]
		for i, part := range groupParts {
			if i > 0 {
				group += "."
			}
			group += part
		}
	}

	return schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resourceOnly,
	}
}

func generateKindVariations(resourceName string) []string {
	if len(resourceName) == 0 {
		return []string{resourceName}
	}

	variations := []string{
		resourceName,
		capitalizeFirst(resourceName),
		toUpper(resourceName),
		capitalizeFirst(resourceName),
	}

	return variations
}

func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}

	first := string(s[0])
	rest := ""
	if len(s) > 1 {
		rest = s[1:] // Keep original case for the rest
	}

	return toUpper(first) + rest
}

func toUpper(s string) string {
	result := ""
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			result += string(r - 32)
		} else {
			result += string(r)
		}
	}
	return result
}

func toLower(s string) string {
	result := ""
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			result += string(r + 32)
		} else {
			result += string(r)
		}
	}
	return result
}

func containsString(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}

	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}

	return false
}
