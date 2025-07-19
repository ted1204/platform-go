package utils

import (
	"encoding/json"
	"strings"

	"gopkg.in/yaml.v2"
)

func ReplacePlaceholders(template string, values map[string]string) string {
	return replaceString(template, values)
}

func replaceString(s string, values map[string]string) string {
	for key, val := range values {
		s = strings.ReplaceAll(s, "{{"+key+"}}", val)
	}
	return s
}

func ReplacePlaceholdersInJSON(jsonStr string, values map[string]string) (string, error) {
	var data interface{}

	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return "", err
	}

	replaced := replaceInInterface(data, values)

	out, err := json.MarshalIndent(replaced, "", "  ")
	if err != nil {
		return "", err
	}

	return string(out), nil
}

func replaceInInterface(v interface{}, values map[string]string) interface{} {
	switch val := v.(type) {
	case string:
		return replaceString(val, values)
	case []interface{}:
		for i := range val {
			val[i] = replaceInInterface(val[i], values)
		}
		return val
	case map[string]interface{}:
		for k, v2 := range val {
			val[k] = replaceInInterface(v2, values)
		}
		return val
	default:
		return v
	}
}

func YAMLToJSON(yamlContent string) (string, error) {
	var yamlObj interface{}
	err := yaml.Unmarshal([]byte(yamlContent), &yamlObj)
	if err != nil {
		return "", err
	}

	jsonReady := convertToStringKeys(yamlObj)

	jsonBytes, err := json.MarshalIndent(jsonReady, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}

func convertToStringKeys(v interface{}) interface{} {
	switch x := v.(type) {
	case map[interface{}]interface{}:
		m2 := make(map[string]interface{})
		for k, v2 := range x {
			m2[k.(string)] = convertToStringKeys(v2)
		}
		return m2
	case []interface{}:
		for i, v2 := range x {
			x[i] = convertToStringKeys(v2)
		}
	}
	return v
}
