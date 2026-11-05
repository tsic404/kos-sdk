// Package fixtures holds sample author declarations used by the acceptance
// cases. They declare only fields defined by the framework contracts.
package fixtures

// MinimalApp returns a minimal window application declaration. Each call
// builds a fresh value so a case can mutate its copy.
func MinimalApp() map[string]any {
	return map[string]any{
		"id":      "files",
		"name":    "Files",
		"version": "1.0.0",
		"icon":    "assets/icon.svg",
		"entry":   "https://files.apps.kos.example/",
		"window": map[string]any{
			"width":        1024,
			"height":       720,
			"minWidth":     640,
			"minHeight":    480,
			"resizable":    true,
			"mode":         "single",
			"defaultState": "normal",
		},
		"permissions": []any{"clipboard-read", "external-link"},
		"backend":     map[string]any{"type": "http", "health": "/healthz"},
		"deepLinks":   []any{"files://open"},
	}
}

// MinimalExtension returns a minimal extension declaration contributing an
// object class, a controller, a page and one App.
func MinimalExtension() map[string]any {
	return map[string]any{
		"id":               "kos.example.files",
		"version":          "1.0.0",
		"frameworkVersion": ">=1.0 <2.0",
		"contributions": map[string]any{
			"objectClasses": []any{
				map[string]any{
					"apiGroup": "files.kos.example",
					"version":  "v1alpha1",
					"kind":     "FileShare",
					"scope":    "Namespaced",
				},
			},
			"controllers": []any{
				map[string]any{
					"name": "files-controller",
					"writeScopes": []any{
						map[string]any{
							"apiGroup": "files.kos.example",
							"kind":     "FileShare",
							"verbs":    []any{"get", "update"},
						},
					},
				},
			},
			"capabilities":       []any{map[string]any{"name": "files.kos.example/share", "version": "v1"}},
			"pages":              []any{map[string]any{"id": "files"}},
			"settings":           []any{},
			"catalogSources":     []any{},
			"discoveryAdapters":  []any{},
			"diagnosticAdapters": []any{},
			"userPreferences":    map[string]any{"properties": map[string]any{"theme": "string"}},
			"apps":               []any{MinimalApp()},
		},
		"dependencies": map[string]any{
			"capabilities": []any{
				map[string]any{
					"name":       "storage.kos.example/volume",
					"version":    ">=1.0",
					"guarantees": map[string]any{"snapshot": true},
				},
			},
			"components": []any{map[string]any{"id": "kos.example.storage", "version": ">=1.0"}},
			"framework":  ">=1.0 <2.0",
		},
		"permissions": map[string]any{
			"write": []any{
				map[string]any{
					"apiGroup":  "files.kos.example",
					"resources": []any{"fileshares"},
					"verbs":     []any{"create", "update"},
				},
			},
			"read": []any{
				map[string]any{"apiGroup": "storage.kos.example", "resources": []any{"volumes"}},
			},
			"roles":       []any{"Member"},
			"objectRoles": map[string]any{"FileShare": []any{"Owner", "Member"}},
		},
		"config": map[string]any{
			"properties": map[string]any{"path": map[string]any{"type": "string"}},
		},
		"status": map[string]any{
			"component": map[string]any{"conditions": []any{"Ready"}},
			"objectClasses": map[string]any{
				"FileShare": map[string]any{
					"writer":     "files-controller",
					"conditions": []any{"Ready"},
				},
			},
		},
		"uninstallRetention": map[string]any{
			"contributions":     map[string]any{"settings": "retain"},
			"externalResources": []any{"files.kos.example/remote"},
		},
	}
}

// MinimalPageExtension returns a minimal extension that contributes one page
// and nothing else.
func MinimalPageExtension() map[string]any {
	declaration := minimalDeclaration("kos.example.pages")
	declaration["contributions"] = map[string]any{
		"pages": []any{map[string]any{"id": "reports"}},
	}
	return declaration
}

// MinimalWindowApp returns a minimal extension whose only contribution is one
// window application.
func MinimalWindowApp() map[string]any {
	declaration := minimalDeclaration("kos.example.notes")
	declaration["contributions"] = map[string]any{
		"apps": []any{minimalApp("notes", "Notes", "assets/icon.svg")},
	}
	return declaration
}

// SystemApp returns a first-party system application: an extension whose only
// contribution is one window application. Nothing in the declaration marks it
// as first party, so the packing path cannot tell it from a third-party one.
func SystemApp() map[string]any {
	declaration := minimalDeclaration("kos.files")
	declaration["contributions"] = map[string]any{
		"apps": []any{minimalApp("files", "Files", "assets/icon.svg")},
	}
	return declaration
}

// ThirdPartyExtension returns a third-party extension contributing one page and
// one window application whose icon lives on the application's own origin.
func ThirdPartyExtension() map[string]any {
	declaration := minimalDeclaration("kos.example.notes")
	declaration["contributions"] = map[string]any{
		"pages": []any{map[string]any{"id": "notes"}},
		"apps":  []any{minimalApp("notes", "Notes", "https://notes.apps.kos.example/icon.svg")},
	}
	return declaration
}

// minimalDeclaration returns a valid declaration skeleton; callers set the
// contributions of their case and leave every other block empty.
func minimalDeclaration(id string) map[string]any {
	return map[string]any{
		"id":                 id,
		"version":            "1.0.0",
		"frameworkVersion":   ">=1.0 <2.0",
		"contributions":      map[string]any{},
		"dependencies":       map[string]any{},
		"permissions":        map[string]any{},
		"config":             map[string]any{},
		"status":             map[string]any{},
		"uninstallRetention": map[string]any{},
	}
}

func minimalApp(id, name, icon string) map[string]any {
	return map[string]any{
		"id":      id,
		"name":    name,
		"version": "1.0.0",
		"icon":    icon,
		"entry":   "https://" + id + ".apps.kos.example/",
		"window": map[string]any{
			"width":        1024,
			"height":       720,
			"minWidth":     640,
			"minHeight":    480,
			"resizable":    true,
			"mode":         "single",
			"defaultState": "normal",
		},
		"permissions": []any{},
	}
}
