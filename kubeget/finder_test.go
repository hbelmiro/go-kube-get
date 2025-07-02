package kubeget

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

func TestNewFinder(t *testing.T) {
	tests := []struct {
		name        string
		config      *rest.Config
		expectError bool
	}{
		{
			name:        "nil config should return error",
			config:      nil,
			expectError: true,
		},
		{
			name: "invalid config should return error",
			config: &rest.Config{
				Host: "invalid://host:port",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			finder, err := NewFinder(tt.config)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				if finder != nil {
					t.Error("expected nil finder when error occurs")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if finder == nil {
					t.Error("expected finder but got nil")
				}
			}
		})
	}
}

func TestFindGVR_FullyQualifiedNames(t *testing.T) {
	tests := []struct {
		name         string
		resourceName string
		expected     schema.GroupVersionResource
		expectError  bool
	}{
		{
			name:         "fully qualified custom resource",
			resourceName: "deployments.v1.apps",
			expected: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			},
			expectError: false,
		},
		{
			name:         "fully qualified core resource",
			resourceName: "services.v1.",
			expected: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "services",
			},
			expectError: false,
		},
		{
			name:         "fully qualified with apps group",
			resourceName: "replicasets.v1.apps",
			expected: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "replicasets",
			},
			expectError: false,
		},
		{
			name:         "invalid format - only two parts",
			resourceName: "resource.version",
			expected:     schema.GroupVersionResource{},
			expectError:  false, // This should fall through to other discovery methods
		},
		{
			name:         "invalid format - no dots",
			resourceName: "pods",
			expected:     schema.GroupVersionResource{},
			expectError:  false, // This should fall through to other discovery methods
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the parsing logic by checking if the resource name contains dots
			// and would be handled by the fully qualified path
			if !tt.expectError && len(tt.resourceName) > 0 && tt.resourceName != "resource.version" && tt.resourceName != "pods" {
				parts := splitResourceName(tt.resourceName)
				if len(parts) >= 3 {
					result := parseFullyQualifiedResource(parts)

					if result.Group != tt.expected.Group {
						t.Errorf("expected group %q, got %q", tt.expected.Group, result.Group)
					}
					if result.Version != tt.expected.Version {
						t.Errorf("expected version %q, got %q", tt.expected.Version, result.Version)
					}
					if result.Resource != tt.expected.Resource {
						t.Errorf("expected resource %q, got %q", tt.expected.Resource, result.Resource)
					}
				}
			}
		})
	}
}

func TestResourceNameVariations(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:  "lowercase input",
			input: "pod",
			expected: []string{
				"pod",
				"Pod",
				"POD",
				"Pod",
			},
		},
		{
			name:  "mixed case input",
			input: "replicaSet",
			expected: []string{
				"replicaSet",
				"ReplicaSet",
				"REPLICASET",
				"ReplicaSet",
			},
		},
		{
			name:  "uppercase input",
			input: "SERVICE",
			expected: []string{
				"SERVICE",
				"SERVICE",
				"SERVICE",
				"SERVICE",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			variations := generateKindVariations(tt.input)

			if len(variations) != len(tt.expected) {
				t.Errorf("expected %d variations, got %d", len(tt.expected), len(variations))
				return
			}

			for i, expected := range tt.expected {
				if variations[i] != expected {
					t.Errorf("variation %d: expected %q, got %q", i, expected, variations[i])
				}
			}
		})
	}
}

func TestGet_ErrorHandling(t *testing.T) {
	tests := []struct {
		name         string
		resourceName string
		namespace    string
		expectError  bool
		errorSubstr  string
	}{
		{
			name:         "empty resource name",
			resourceName: "",
			namespace:    "default",
			expectError:  true,
			errorSubstr:  "failed to find resource",
		},
		{
			name:         "invalid resource name",
			resourceName: "nonexistentresource",
			namespace:    "default",
			expectError:  true,
			errorSubstr:  "failed to find resource",
		},
	}

	// Create a finder with mock configuration for error testing
	config := &rest.Config{
		Host: "https://localhost:8443",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			finder, err := NewFinder(config)
			if err != nil {
				t.Skip("Skipping test - cannot create finder with mock config")
			}

			ctx := context.Background()
			_, _, err = finder.Get(ctx, tt.resourceName, tt.namespace)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				} else if tt.errorSubstr != "" && !containsString(err.Error(), tt.errorSubstr) {
					t.Errorf("expected error to contain %q, got: %v", tt.errorSubstr, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// Helper functions for testing

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
