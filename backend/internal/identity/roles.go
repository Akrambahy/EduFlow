package identity

const (
	RoleAdmin      = "admin"
	RoleInstructor = "instructor"
	RoleMentor     = "mentor"
	RoleTracker    = "tracker"
	RoleStudent    = "student"
)

var rolePriority = []string{RoleAdmin, RoleInstructor, RoleMentor, RoleTracker, RoleStudent}

func IsValidRole(name string) bool {
	for _, r := range rolePriority {
		if r == name {
			return true
		}
	}
	return false
}

func PrimaryRole(roles []string) string {
	for _, r := range rolePriority {
		for _, have := range roles {
			if have == r {
				return r
			}
		}
	}
	return ""
}

func SortRoles(roles []string) []string {
	out := make([]string, 0, len(roles))
	for _, r := range rolePriority {
		for _, have := range roles {
			if have == r {
				out = append(out, r)
				break
			}
		}
	}
	return out
}
