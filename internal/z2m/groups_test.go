package z2m

import "testing"

func TestGroupNames(t *testing.T) {
	names, err := groupNames([]byte(`[{"id":1,"friendly_name":"living-room","members":[]},{"id":2,"friendly_name":"hallway"}]`))
	if err != nil || len(names) != 2 || names[0] != "living-room" || names[1] != "hallway" {
		t.Errorf("names %v, err %v", names, err)
	}
	if _, err := groupNames([]byte(`{"oops":1}`)); err == nil {
		t.Error("not a list of groups")
	}
}
