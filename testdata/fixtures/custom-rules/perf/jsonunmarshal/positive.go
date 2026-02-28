package jsonunmarshal

import "encoding/json"

func Positive(data []byte) {
	var first map[string]any
	var second map[string]any
	_ = json.Unmarshal(data, &first)
	_ = json.Unmarshal(data, &second) // want perf/json-unmarshal-twice
}
