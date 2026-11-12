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
// object class, a controller, a capability, a page and a preference set.
func MinimalExtension() map[string]any {
	declaration := minimalDeclaration("kos.example.files")
	declaration["apiGroup"] = "files.kos.example"
	declaration["contributions"] = map[string]any{
		"objectClasses": []any{
			map[string]any{
				"group":      "files.kos.example",
				"kind":       "FileShare",
				"resource":   "fileshares",
				"versions":   []any{"v1alpha1"},
				"namespaced": true,
			},
		},
		"controllers": []any{
			map[string]any{
				"id":      "files-controller",
				"manages": []any{map[string]any{"group": "files.kos.example", "kind": "FileShare"}},
				"scope": map[string]any{
					"groups": []any{"files.kos.example"},
					"kinds":  []any{map[string]any{"kind": "ConfigMap"}},
				},
			},
		},
		"capabilities": map[string]any{
			"items": []any{map[string]any{"name": "kos.example.files/share", "version": "v1"}},
		},
		"pages":              []any{readOnlyPage("files", "Files", "files.kos.example/v1alpha1/FileShare")},
		"settings":           []any{},
		"catalogueSources":   []any{},
		"discoveryAdapters":  []any{},
		"diagnosticAdapters": []any{},
		"preferences":        []any{map[string]any{"id": "files.view", "keys": []any{"sort"}}},
	}
	declaration["dependencies"] = map[string]any{
		"capabilities": []any{map[string]any{"name": "storage.kos.example/volume", "versions": ">=1.0"}},
		"components":   []any{map[string]any{"identifier": "kos.example.storage", "versions": ">=1.0"}},
	}
	declaration["permissions"] = []any{
		map[string]any{
			"contribution": "FileShare",
			"readScope": map[string]any{
				"groups": []any{"files.kos.example"},
				"kinds": []any{
					map[string]any{"group": "storage.kos.example", "kind": "Volume"},
					map[string]any{"kind": "Event"},
				},
			},
			"requiredRole":  "Member",
			"objectWriters": []any{"Owner", "Member"},
		},
		map[string]any{"contribution": "files", "requiredRole": "Guest"},
	}
	declaration["configuration"] = map[string]any{
		"schema":        map[string]any{"properties": map[string]any{"path": map[string]any{"type": "string"}}},
		"statusWriters": []any{"Member"},
		"conditions":    []any{"Ready"},
	}
	declaration["retention"] = []any{
		map[string]any{"resource": "settings", "keep": true},
		map[string]any{"resource": "fileshares", "keep": false, "externalReference": "files.kos.example/remote"},
	}
	return declaration
}

// CoreGroupReadExtension returns a minimal extension whose only read scope is a
// core-group kind: the group field is absent, as the kernel declares it.
func CoreGroupReadExtension() map[string]any {
	declaration := minimalDeclaration("kos.example.events")
	declaration["permissions"] = []any{
		map[string]any{
			"contribution": "events",
			"readScope": map[string]any{
				"kinds": []any{map[string]any{"kind": "Event"}},
			},
		},
	}
	return declaration
}

// MinimalPageExtension returns a minimal extension that contributes one page
// and nothing else.
func MinimalPageExtension() map[string]any {
	declaration := minimalDeclaration("kos.example.pages")
	declaration["contributions"] = map[string]any{
		"pages": []any{readOnlyPage("reports", "Reports", "kos.example/v1/Report")},
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
		"pages": []any{readOnlyPage("notes", "Notes", "kos.example/v1/Note")},
		"apps":  []any{minimalApp("notes", "Notes", "https://notes.apps.kos.example/icon.svg")},
	}
	return declaration
}

// minimalDeclaration returns a valid declaration skeleton; callers set the
// contributions of their case and leave every other block empty.
func minimalDeclaration(identifier string) map[string]any {
	return map[string]any{
		"identifier":       identifier,
		"version":          "1.0.0",
		"frameworkVersion": ">=1.0 <2.0",
		"contributions":    map[string]any{},
		"dependencies":     map[string]any{},
		"permissions":      []any{},
		"configuration":    map[string]any{},
		"retention":        []any{},
	}
}

// readOnlyPage returns a page whose single item reads one object source; a
// read-only page declares why and carries no action (KOSD-C-22, KOSD-C-53).
func readOnlyPage(id, title, reference string) map[string]any {
	return map[string]any{
		"id":             id,
		"title":          title,
		"location":       "panel",
		"readOnly":       true,
		"readOnlyReason": "the source is read-only in this release",
		"items": []any{
			map[string]any{"id": id + "-items", "label": title, "source": "object", "reference": reference},
		},
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
