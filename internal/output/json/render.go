package json

import (
	"encoding/json"
	"fmt"

	"github.com/stanislavstehniy/go-doctor/pkg/godoctor"
)

func Render(result godoctor.DiagnoseResult) ([]byte, error) {
	body, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal json output: %w", err)
	}
	return append(body, '\n'), nil
}
