package adminpolicy

// Retired codes stay reserved so they cannot be recreated as platform delegates.
func IsRetiredTempleRole(role string) bool {
	return role == "temple_reception" || role == "temple_operations" || role == "temple_finance"
}

func IsTempleRole(role string) bool { return role == "temple_admin" }
