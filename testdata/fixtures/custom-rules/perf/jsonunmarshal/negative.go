package jsonunmarshal

import "encoding/json"

func Negative(data []byte) {
	var first map[string]any
	_ = json.Unmarshal(data, &first)
}
