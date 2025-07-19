package unit_tests

import (
	"log"
	"testing"

	"github.com/linskybing/platform-go/utils"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestValidateK8sJSON(t *testing.T) {
	validPodJSON := `{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "test-pod"
		},
		"spec": {}
	}`

	gvk, name, err := utils.ValidateK8sJSON(validPodJSON)
	log.Print(name)
	assert.NoError(t, err, "should decode valid Pod JSON without error")
	assert.NotNil(t, gvk)
	assert.Equal(t, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}, *gvk)
	assert.Equal(t, "test-pod", name)

	invalidJSON := `{"apiVersion": "v1", "kind": "Pod", "metadata": { "name": "bad-pod" },`

	_, _, err = utils.ValidateK8sJSON(invalidJSON)
	assert.Error(t, err, "should return error for invalid JSON")

	unknownKindJSON := `{
		"apiVersion": "v1",
		"kind": "UnknownKind",
		"metadata": {
			"name": "unknown"
		}
	}`

	_, _, err = utils.ValidateK8sJSON(unknownKindJSON)
	assert.Error(t, err, "should return error for unknown kind")
}
