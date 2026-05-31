package api

import (
	"encoding/json"
	"fmt"
)

// JSONString accepts JSON string or number (Web UI number inputs may send numeric ports).
type JSONString string

func (s *JSONString) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*s = ""
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		var v string
		if err := json.Unmarshal(b, &v); err != nil {
			return err
		}
		*s = JSONString(v)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return fmt.Errorf("json string or number expected")
	}
	*s = JSONString(n.String())
	return nil
}

func (s JSONString) String() string {
	return string(s)
}
