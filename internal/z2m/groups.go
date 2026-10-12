package z2m

import "encoding/json"

func groupNames(p []byte) ([]string, error) {
	var gs []struct {
		Name string `json:"friendly_name"`
	}
	if err := json.Unmarshal(p, &gs); err != nil {
		return nil, err
	}
	names := make([]string, len(gs))
	for i, g := range gs {
		names[i] = g.Name
	}
	return names, nil
}
