// Package manifest carries the extension declaration of KOSD-C-18 and the App
// manifest of KOSD-C-73. Both field sets are fixed by those contracts; the SDK
// adds no field of its own and fills no default value.
package manifest

import (
	"net/url"

	"github.com/tsic404/kos-sdk/validate"
)

// AppManifestFields is the App manifest's closed field set (KOSD-C-73).
var AppManifestFields = []string{
	"id",
	"name",
	"version",
	"icon",
	"entry",
	"window",
	"permissions",
	"backend",
	"deepLinks",
}

// AppWindowFields is the window declaration's closed field set (KOSD-C-73).
var AppWindowFields = []string{
	"width",
	"height",
	"minWidth",
	"minHeight",
	"resizable",
	"mode",
	"defaultState",
}

// AppBackendFields is the backend declaration's closed field set (KOSD-C-73).
var AppBackendFields = []string{"type", "health"}

// AppWindow is the window declaration; sizes are pixels.
type AppWindow struct {
	Width        float64 `json:"width"`
	Height       float64 `json:"height"`
	MinWidth     float64 `json:"minWidth"`
	MinHeight    float64 `json:"minHeight"`
	Resizable    bool    `json:"resizable"`
	Mode         string  `json:"mode"`
	DefaultState string  `json:"defaultState"`
}

// AppBackend is the optional application backend; its absence means UI-only.
type AppBackend struct {
	Type   string `json:"type"`
	Health string `json:"health"`
}

// AppManifest is an independent UI document declaration, not a Page variant.
type AppManifest struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Version     string       `json:"version"`
	Icon        string       `json:"icon"`
	Entry       string       `json:"entry"`
	Window      AppWindow    `json:"window"`
	Permissions []Permission `json:"permissions"`
	Backend     *AppBackend  `json:"backend,omitempty"`
	DeepLinks   []string     `json:"deepLinks,omitempty"`
}

// ValidateAppManifest checks an App manifest and returns it unchanged.
func ValidateAppManifest(input any) validate.Result[AppManifest] {
	c := &validate.Checker{}
	app := checkApp(c, input, "$")
	if len(c.Issues) > 0 {
		return validate.Result[AppManifest]{Issues: c.Issues}
	}
	return validate.Result[AppManifest]{Manifest: &app}
}

func checkApp(c *validate.Checker, value any, path string) AppManifest {
	record, ok := c.Record(value, path)
	if !ok {
		return AppManifest{}
	}
	c.RejectUnknown(record, AppManifestFields, path)

	app := AppManifest{}
	app.ID, _ = c.RequireString(record, "id", path)
	app.Name, _ = c.RequireString(record, "name", path)
	app.Version, _ = c.RequireString(record, "version", path)
	app.Icon, _ = c.RequireString(record, "icon", path)
	app.Entry = checkEntry(c, record, path)
	app.Window = checkWindowField(c, record, path)
	app.Permissions = checkAppPermissionsField(c, record, path)
	app.Backend = checkBackend(c, record, path)
	app.DeepLinks = checkDeepLinks(c, record, path)
	return app
}

// KOSD-C-73 fixes entry as an HTTPS address on an origin of its own.
func checkEntry(c *validate.Checker, record map[string]any, path string) string {
	entry, ok := c.RequireString(record, "entry", path)
	if !ok {
		return ""
	}
	parsed, err := url.Parse(entry)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		c.Add(validate.Join(path, "entry"), validate.CodeInvalidValue,
			"entry must be an absolute https URL (KOSD-C-73)")
		return ""
	}
	if parsed.User != nil {
		c.Add(validate.Join(path, "entry"), validate.CodeInvalidValue,
			"entry is an origin address and must not carry credentials (KOSD-C-73)")
		return ""
	}
	return entry
}

func checkWindowField(c *validate.Checker, record map[string]any, path string) AppWindow {
	value, ok := c.Field(record, "window", path)
	if !ok {
		return AppWindow{}
	}
	windowPath := validate.Join(path, "window")
	window, ok := c.Record(value, windowPath)
	if !ok {
		return AppWindow{}
	}
	c.RejectUnknown(window, AppWindowFields, windowPath)

	declaration := AppWindow{}
	declaration.Width, _ = c.RequireNumber(window, "width", windowPath, true)
	declaration.Height, _ = c.RequireNumber(window, "height", windowPath, true)
	declaration.MinWidth, _ = c.RequireNumber(window, "minWidth", windowPath, true)
	declaration.MinHeight, _ = c.RequireNumber(window, "minHeight", windowPath, true)
	declaration.Resizable, _ = c.RequireBoolean(window, "resizable", windowPath)
	declaration.Mode, _ = c.RequireString(window, "mode", windowPath)
	declaration.DefaultState, _ = c.RequireString(window, "defaultState", windowPath)
	return declaration
}

func checkAppPermissionsField(c *validate.Checker, record map[string]any, path string) []Permission {
	value, ok := c.Field(record, "permissions", path)
	if !ok {
		return nil
	}
	// KOSD-C-73 requires the field and allows it to be empty; a present null is
	// the empty set, not a missing field.
	if value == nil {
		return []Permission{}
	}
	permissionsPath := validate.Join(path, "permissions")
	items, ok := c.Array(value, permissionsPath)
	if !ok {
		return nil
	}
	permissions := make([]Permission, 0, len(items))
	for index, item := range items {
		itemPath := validate.Index(permissionsPath, index)
		name, ok := c.String(item, itemPath)
		if !ok {
			continue
		}
		if !isAppPermission(name) {
			c.Add(itemPath, validate.CodePermissionNotInSet,
				"%q is not in the fixed permission set %v", name, AppPermissions)
			continue
		}
		permissions = append(permissions, Permission(name))
	}
	return permissions
}

func checkBackend(c *validate.Checker, record map[string]any, path string) *AppBackend {
	value, ok := c.Optional(record, "backend")
	if !ok {
		return nil
	}
	backendPath := validate.Join(path, "backend")
	backend, ok := c.Record(value, backendPath)
	if !ok {
		return nil
	}
	c.RejectUnknown(backend, AppBackendFields, backendPath)

	declaration := &AppBackend{}
	declaration.Type, _ = c.RequireString(backend, "type", backendPath)
	declaration.Health, _ = c.RequireString(backend, "health", backendPath)
	return declaration
}

func checkDeepLinks(c *validate.Checker, record map[string]any, path string) []string {
	value, ok := c.Optional(record, "deepLinks")
	if !ok {
		return nil
	}
	linksPath := validate.Join(path, "deepLinks")
	items, ok := c.Array(value, linksPath)
	if !ok {
		return nil
	}
	links := make([]string, 0, len(items))
	for index, item := range items {
		link, ok := c.String(item, validate.Index(linksPath, index))
		if !ok {
			continue
		}
		links = append(links, link)
	}
	return links
}
