package identity

import (
	"reflect"
	"testing"
)

func TestPrimaryRole(t *testing.T) {
	cases := []struct {
		roles []string
		want  string
	}{
		{[]string{"student"}, "student"},
		{[]string{"student", "mentor"}, "mentor"},
		{[]string{"tracker", "admin", "student"}, "admin"},
		{[]string{"unknown"}, ""},
		{nil, ""},
	}
	for _, c := range cases {
		if got := PrimaryRole(c.roles); got != c.want {
			t.Errorf("PrimaryRole(%v) = %q, want %q", c.roles, got, c.want)
		}
	}
}

func TestSortRoles(t *testing.T) {
	got := SortRoles([]string{"student", "bogus", "instructor"})
	want := []string{"instructor", "student"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SortRoles = %v, want %v", got, want)
	}
}

func TestIsValidRole(t *testing.T) {
	if !IsValidRole("tracker") || IsValidRole("Admin") || IsValidRole("") {
		t.Fatal("IsValidRole mismatch")
	}
}
