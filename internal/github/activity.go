package github

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// Source activity excludes polling times and normalizes unordered API collections.
func activityToken(value any) string {
	b, err := json.Marshal(value)
	if err != nil {
		panic(err)
	} // All inputs are JSON data models, without unsupported values.
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	var v any
	if err = decoder.Decode(&v); err != nil {
		panic(err)
	}
	b, _ = json.Marshal(canonicalActivity(v))
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:16])
}

func canonicalActivity(value any) any {
	switch v := value.(type) {
	case []any:
		for i := range v {
			v[i] = canonicalActivity(v[i])
		}
		sort.Slice(v, func(i, j int) bool {
			a, _ := json.Marshal(v[i])
			b, _ := json.Marshal(v[j])
			return string(a) < string(b)
		})
	case map[string]any:
		for k, x := range v {
			v[k] = canonicalActivity(x)
		}
	}
	return value
}
