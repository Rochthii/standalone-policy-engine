package storage

import "testing"

func TestValidateRoleInheritances(t *testing.T) {
	valid := [][2]string{{"user:alice", "role:manager"}, {"role:manager", "role:staff"}}
	if err := validateRoleInheritances(valid); err != nil {
		t.Fatalf("valid role DAG rejected: %v", err)
	}

	for _, test := range []struct {
		name  string
		edges [][2]string
	}{
		{name: "self edge", edges: [][2]string{{"role:a", "role:a"}}},
		{name: "duplicate", edges: [][2]string{{"role:a", "role:b"}, {"role:a", "role:b"}}},
		{name: "cycle", edges: [][2]string{{"role:a", "role:b"}, {"role:b", "role:c"}, {"role:c", "role:a"}}},
		{name: "untrimmed", edges: [][2]string{{" role:a", "role:b"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := validateRoleInheritances(test.edges); err == nil {
				t.Fatal("invalid role graph was accepted")
			}
		})
	}
}
