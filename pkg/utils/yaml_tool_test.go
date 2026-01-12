package utils

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitYAMLDocuments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name: "single document",
			input: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: config1
`,
			expected: []string{
				`apiVersion: v1
kind: ConfigMap
metadata:
  name: config1`,
			},
		},
		{
			name: "multiple documents with --- separator",
			input: `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: config1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: config2
`,
			expected: []string{
				`apiVersion: v1
kind: ConfigMap
metadata:
  name: config1`,
				`apiVersion: v1
kind: ConfigMap
metadata:
  name: config2`,
			},
		},
		{
			name: "trailing separator",
			input: `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: config1
---
`,
			expected: []string{
				`apiVersion: v1
kind: ConfigMap
metadata:
  name: config1`,
			},
		},
		{
			name:     "empty input",
			input:    ``,
			expected: []string{},
		},
		{
			name:     "only separators",
			input:    `---`,
			expected: []string{},
		},
		{
			name: "yaml with --- in string content (Pod with python script)",
			input: `apiVersion: v1
kind: Pod
metadata:
  name: gpu-test-5090
spec:
  restartPolicy: Never
  containers:
  - name: pytorch-ngc
    image: nvcr.io/nvidia/pytorch:24.12-py3
    resources:
      limits:
        nvidia.com/gpu: 1
    command: ["python", "-c"]
    args:
    - |
      import torch
      import sys
      
      print("--- Environment Check ---")
      print(f"Python: {sys.version.split()[0]}")
      print(f"PyTorch: {torch.__version__}")
      
      if torch.cuda.is_available():
          print("\n--- GPU Details ---")
          device_id = 0
          props = torch.cuda.get_device_properties(device_id)
          print(f"Device Name: {props.name}")
          
          print("\n--- Performance Test (Matrix Mul) ---")
          try:
              size = 8192
              print(f"Initializing {size}x{size} tensors...")
              x = torch.randn(size, size, device='cuda')
              print(f"Success! Result shape: {x.shape}")
          except Exception as e:
              print(f"Calculation Failed: {e}")
      else:
          print("Error: CUDA is not available.")`,
			expected: []string{
				`apiVersion: v1
kind: Pod
metadata:
  name: gpu-test-5090
spec:
  restartPolicy: Never
  containers:
  - name: pytorch-ngc
    image: nvcr.io/nvidia/pytorch:24.12-py3
    resources:
      limits:
        nvidia.com/gpu: 1
    command: ["python", "-c"]
    args:
    - |
      import torch
      import sys
      
      print("--- Environment Check ---")
      print(f"Python: {sys.version.split()[0]}")
      print(f"PyTorch: {torch.__version__}")
      
      if torch.cuda.is_available():
          print("\n--- GPU Details ---")
          device_id = 0
          props = torch.cuda.get_device_properties(device_id)
          print(f"Device Name: {props.name}")
          
          print("\n--- Performance Test (Matrix Mul) ---")
          try:
              size = 8192
              print(f"Initializing {size}x{size} tensors...")
              x = torch.randn(size, size, device='cuda')
              print(f"Success! Result shape: {x.shape}")
          except Exception as e:
              print(f"Calculation Failed: {e}")
      else:
          print("Error: CUDA is not available.")`,
			},
		},
		{
			name: "multiple documents with --- in content",
			input: `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: config1
data:
  message: "--- This is not a separator ---"
---
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
  - name: test
    args:
    - "echo '--- Still not a separator ---'"`,
			expected: []string{
				`apiVersion: v1
kind: ConfigMap
metadata:
  name: config1
data:
  message: "--- This is not a separator ---"`,
				`apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
  - name: test
    args:
    - "echo '--- Still not a separator ---'"`,
			},
		},
		{
			name: "yaml with --- comment",
			input: `apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    description: "--- This annotation has dashes"
spec:
  selector:
    app: myapp`,
			expected: []string{
				`apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    description: "--- This annotation has dashes"
spec:
  selector:
    app: myapp`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := SplitYAMLDocuments(tt.input)
			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("SplitYAMLDocuments() = %v, want %v", actual, tt.expected)
			}
		})
	}
}

func TestReplacePlaceholders(t *testing.T) {
	template := "apiVersion: v1\nkind: Pod\nmetadata:\n  name: {{name}}\nspec:\n  image: {{image}}"
	values := map[string]string{
		"name":  "my-pod",
		"image": "nginx:latest",
	}

	expected := "apiVersion: v1\nkind: Pod\nmetadata:\n  name: my-pod\nspec:\n  image: nginx:latest"
	result := ReplacePlaceholders(template, values)

	assert.Equal(t, expected, result)
}

func TestReplacePlaceholdersInJSON(t *testing.T) {
	jsonStr := `{
		"metadata": {
			"name": "{{name}}"
		},
		"spec": {
			"containers": [{
				"image": "{{image}}"
			}]
		}
	}`

	values := map[string]string{
		"name":  "my-app",
		"image": "nginx:1.25",
	}

	result, err := ReplacePlaceholdersInJSON(jsonStr, values)
	assert.NoError(t, err)
	assert.Contains(t, result, `"name": "my-app"`)
	assert.Contains(t, result, `"image": "nginx:1.25"`)
}

func TestYAMLToJSON(t *testing.T) {
	yamlContent := `
apiVersion: v1
kind: Pod
metadata:
  name: my-pod
spec:
  containers:
    - name: nginx
      image: nginx:1.25
`

	expectedContains := `"kind": "Pod"`
	jsonBytes, err := YAMLToJSON([]byte(yamlContent))
	assert.NoError(t, err)
	assert.Contains(t, string(jsonBytes), expectedContains)
}
