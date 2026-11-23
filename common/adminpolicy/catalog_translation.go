package adminpolicy

import "strings"

const CatalogTranslationPrefix = "/api/v1/ai/admin/catalog-translations/"

var catalogTranslationPermissions = map[string]string{
	"shop-product":    "platform.products",
	"points-product":  "platform.points",
	"reward-campaign": "platform.marketing",
	"reward-prize":    "platform.marketing",
	"diy-material":    "platform.diy",
	"diy-design":      "platform.diy",
	"temple":          "platform.temples",
	"temple-service":  "platform.temples",
	"master":          "platform.masters",
	"belief":          "super.standards",
}

func CatalogTranslationKind(path string) (string, bool) {
	suffix, ok := strings.CutPrefix(path, CatalogTranslationPrefix)
	if !ok {
		return "", false
	}
	kind, id, found := strings.Cut(suffix, "/")
	return kind, found && id != "" && !strings.Contains(id, "/") && catalogTranslationPermissions[kind] != ""
}

func catalogTranslationRoute(method, path string) (Route, bool) {
	kind, ok := CatalogTranslationKind(path)
	if !ok {
		return Route{}, false
	}
	action := ""
	switch method {
	case "GET":
		action = ".read"
	case "POST", "PUT":
		action = ".write"
	default:
		return Route{}, false
	}
	permission := catalogTranslationPermissions[kind]
	if strings.HasPrefix(permission, "platform.") {
		permission += action
	}
	return Route{Method: method, Template: CatalogTranslationPrefix + kind + "/:id", Permission: permission}, true
}

// CatalogTranslationAccess authorizes the same business roles as source
// management. The catalog handler also checks ownership for each temple record.
func CatalogTranslationAccess(method, path, userType, clientID, scope string, roles, permissions []string) bool {
	route, ok := catalogTranslationRoute(method, path)
	if !ok || userType != "admin" || len(roles) != 1 {
		return false
	}
	role := roles[0]
	switch clientID {
	case "platform-admin":
		if role == "platform_super" {
			return true
		}
		if scope != "platform" || Builtin(role) || !Assignable(route.Permission) {
			return false
		}
		for _, permission := range permissions {
			if permission == route.Permission ||
				(method == "GET" && permission == strings.TrimSuffix(route.Permission, ".read")+".write") {
				return true
			}
		}
	case "shop-admin":
		resource := strings.TrimSuffix(strings.TrimSuffix(route.Permission, ".read"), ".write")
		return role == "shop_admin" && (resource == "platform.products" || resource == "platform.points" || resource == "platform.diy")
	case "temple-admin":
		resource := strings.TrimSuffix(strings.TrimSuffix(route.Permission, ".read"), ".write")
		return IsTempleRole(role) && (resource == "platform.temples" || resource == "platform.masters")
	}
	return false
}
