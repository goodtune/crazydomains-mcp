package api

import (
	"bytes"
	"encoding/json"
	"sort"
)

// redactedKeys are removed from every JSON response body, at any depth,
// before it is decoded. auth_key is the EPP transfer code for a domain.
var redactedKeys = map[string]struct{}{
	"auth_key": {},
}

// redactJSON parses raw as JSON, removes redactedKeys at every level and
// re-encodes it. Numbers are preserved exactly. It returns an error if raw is
// not valid JSON.
func redactJSON(raw []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return json.Marshal(strip(v))
}

func strip(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, e := range t {
			if _, bad := redactedKeys[k]; bad {
				delete(t, k)
				continue
			}
			t[k] = strip(e)
		}
	case []any:
		for i, e := range t {
			t[i] = strip(e)
		}
	}
	return v
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
