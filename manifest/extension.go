package manifest

import (
	"sort"

	"github.com/tsic404/kos-sdk/extension"
	"github.com/tsic404/kos-sdk/validate"
)

// ExtensionManifestFields is the declaration's closed top-level field set; each
// entry mirrors one category of KOSD-C-18.
var ExtensionManifestFields = []string{
	"id",
	"version",
	"frameworkVersion",
	"contributions",
	"dependencies",
	"permissions",
	"config",
	"status",
	"uninstallRetention",
}

// ContributionClasses are the ten contribution classes of KOSD-C-18.
var ContributionClasses = []string{
	"objectClasses",
	"controllers",
	"capabilities",
	"pages",
	"settings",
	"catalogSources",
	"discoveryAdapters",
	"diagnosticAdapters",
	"userPreferences",
	"apps",
}

// DependencyFields is the dependency block's closed field set.
var DependencyFields = []string{"capabilities", "components", "framework"}

// ExtensionPermissionFields is the permission block's closed field set.
var ExtensionPermissionFields = []string{"write", "read", "roles", "objectRoles"}

// WriteScopeFields is one API write scope's closed field set (KOSD-C-18).
var WriteScopeFields = []string{"apiGroup", "resources", "verbs"}

// ReadScopeFields is one API read scope's closed field set (KOSD-C-18).
var ReadScopeFields = []string{"apiGroup", "resources"}

// StatusFields is the status declaration's closed field set.
var StatusFields = []string{"component", "objectClasses"}

// ComponentStatusFields is the component status interface's closed field set.
var ComponentStatusFields = []string{"conditions"}

// ObjectStatusFields is one object class status declaration's closed field set.
var ObjectStatusFields = []string{"writer", "conditions"}

// RetentionFields is the uninstall retention block's closed field set.
var RetentionFields = []string{"contributions", "externalResources"}

// KosRoles are the roles of the KOS role model; an extension object may widen
// write access to them (KOSD-C-18).
var KosRoles = []string{"Owner", "Member", "Guest"}

// Role is one KOS role.
type Role string

const (
	RoleOwner  Role = "Owner"
	RoleMember Role = "Member"
	RoleGuest  Role = "Guest"
)

// RetentionAction is what uninstall does with a contribution's data.
type RetentionAction string

const (
	Retain RetentionAction = "retain"
	Delete RetentionAction = "delete"
)

// Contributions carries the ten contribution classes. A class the author did
// not declare is absent from the produced manifest.
type Contributions struct {
	ObjectClasses      []extension.ObjectClassDeclaration `json:"objectClasses,omitempty"`
	Controllers        []extension.ControllerDeclaration  `json:"controllers,omitempty"`
	Capabilities       []map[string]any                   `json:"capabilities,omitempty"`
	Pages              []map[string]any                   `json:"pages,omitempty"`
	Settings           []map[string]any                   `json:"settings,omitempty"`
	CatalogSources     []map[string]any                   `json:"catalogSources,omitempty"`
	DiscoveryAdapters  []map[string]any                   `json:"discoveryAdapters,omitempty"`
	DiagnosticAdapters []map[string]any                   `json:"diagnosticAdapters,omitempty"`
	UserPreferences    map[string]any                     `json:"userPreferences,omitempty"`
	Apps               []AppManifest                      `json:"apps,omitempty"`
}

// CapabilityDependency is a capability requirement with its version range and
// required guarantees.
type CapabilityDependency struct {
	Name       string         `json:"name"`
	Version    string         `json:"version,omitempty"`
	Guarantees map[string]any `json:"guarantees,omitempty"`
	Optional   bool           `json:"optional,omitempty"`
}

// ComponentDependency requires another installed component.
type ComponentDependency struct {
	ID      string `json:"id"`
	Version string `json:"version,omitempty"`
}

// Dependencies is the dependency block of the declaration.
type Dependencies struct {
	Capabilities []CapabilityDependency `json:"capabilities,omitempty"`
	Components   []ComponentDependency  `json:"components,omitempty"`
	Framework    string                 `json:"framework,omitempty"`
}

// WriteScope grants writes on an API group's resources with a verb set.
type WriteScope struct {
	APIGroup  string   `json:"apiGroup"`
	Resources []string `json:"resources"`
	Verbs     []string `json:"verbs"`
}

// ReadScope grants reads on an API group's resources.
type ReadScope struct {
	APIGroup  string   `json:"apiGroup"`
	Resources []string `json:"resources"`
}

// ExtensionPermissions is the permission block of the declaration.
type ExtensionPermissions struct {
	Write       []WriteScope      `json:"write,omitempty"`
	Read        []ReadScope       `json:"read,omitempty"`
	Roles       []string          `json:"roles,omitempty"`
	ObjectRoles map[string][]Role `json:"objectRoles,omitempty"`
}

// ComponentStatus is the component status interface.
type ComponentStatus struct {
	Conditions []string `json:"conditions,omitempty"`
}

// ObjectStatus names the writer and condition set of one extension object class.
type ObjectStatus struct {
	Writer     string   `json:"writer"`
	Conditions []string `json:"conditions"`
}

// StatusDeclaration is the status block of the declaration.
type StatusDeclaration struct {
	Component     *ComponentStatus        `json:"component,omitempty"`
	ObjectClasses map[string]ObjectStatus `json:"objectClasses,omitempty"`
}

// UninstallRetention fixes what uninstall keeps or removes for each contribution.
type UninstallRetention struct {
	Contributions     map[string]RetentionAction `json:"contributions,omitempty"`
	ExternalResources []string                   `json:"externalResources,omitempty"`
}

// ExtensionManifest is the extension declaration of KOSD-C-18.
type ExtensionManifest struct {
	ID                 string               `json:"id"`
	Version            string               `json:"version"`
	FrameworkVersion   string               `json:"frameworkVersion"`
	Contributions      Contributions        `json:"contributions"`
	Dependencies       Dependencies         `json:"dependencies"`
	Permissions        ExtensionPermissions `json:"permissions"`
	Config             map[string]any       `json:"config"`
	Status             StatusDeclaration    `json:"status"`
	UninstallRetention UninstallRetention   `json:"uninstallRetention"`
}

// ValidateExtensionManifest checks an extension declaration and returns it unchanged.
func ValidateExtensionManifest(input any) validate.Result[ExtensionManifest] {
	c := &validate.Checker{}
	manifest := checkExtension(c, input, "$")
	if len(c.Issues) > 0 {
		return validate.Result[ExtensionManifest]{Issues: c.Issues}
	}
	return validate.Result[ExtensionManifest]{Manifest: &manifest}
}

func checkExtension(c *validate.Checker, value any, path string) ExtensionManifest {
	record, ok := c.Record(value, path)
	if !ok {
		return ExtensionManifest{}
	}
	c.RejectUnknown(record, ExtensionManifestFields, path)

	manifest := ExtensionManifest{}
	manifest.ID, _ = c.RequireString(record, "id", path)
	manifest.Version, _ = c.RequireString(record, "version", path)
	manifest.FrameworkVersion, _ = c.RequireString(record, "frameworkVersion", path)
	manifest.Contributions = checkContributionsField(c, record, path)
	manifest.Dependencies = checkDependenciesField(c, record, path)
	manifest.Permissions = checkExtensionPermissionsField(c, record, path)
	manifest.Config, _ = c.RequireRecord(record, "config", path)
	manifest.Status = checkStatusField(c, record, path)
	manifest.UninstallRetention = checkRetentionField(c, record, path)
	return manifest
}

func checkContributionsField(c *validate.Checker, record map[string]any, path string) Contributions {
	value, ok := c.Field(record, "contributions", path)
	if !ok {
		return Contributions{}
	}
	contributionsPath := validate.Join(path, "contributions")
	declared, ok := c.Record(value, contributionsPath)
	if !ok {
		return Contributions{}
	}
	c.RejectUnknown(declared, ContributionClasses, contributionsPath)

	contributions := Contributions{}
	contributions.ObjectClasses = checkList(c, declared, "objectClasses", contributionsPath, extension.CheckObjectClass)
	contributions.Controllers = checkList(c, declared, "controllers", contributionsPath, extension.CheckController)
	contributions.Apps = checkList(c, declared, "apps", contributionsPath, checkApp)
	contributions.Capabilities = checkList(c, declared, "capabilities", contributionsPath, checkPayload)
	contributions.Pages = checkList(c, declared, "pages", contributionsPath, checkPayload)
	contributions.Settings = checkList(c, declared, "settings", contributionsPath, checkPayload)
	contributions.CatalogSources = checkList(c, declared, "catalogSources", contributionsPath, checkPayload)
	contributions.DiscoveryAdapters = checkList(c, declared, "discoveryAdapters", contributionsPath, checkPayload)
	contributions.DiagnosticAdapters = checkList(c, declared, "diagnosticAdapters", contributionsPath, checkPayload)
	contributions.UserPreferences, _ = c.OptionalRecord(declared, "userPreferences", contributionsPath)
	return contributions
}

// checkPayload accepts an author payload of a contribution class whose fields
// belong to its own contract, not to this manifest.
func checkPayload(c *validate.Checker, value any, path string) map[string]any {
	payload, ok := c.Record(value, path)
	if !ok {
		return nil
	}
	return payload
}

func checkList[T any](
	c *validate.Checker,
	record map[string]any,
	key string,
	path string,
	check func(*validate.Checker, any, string) T,
) []T {
	items, ok := c.OptionalArray(record, key, path)
	if !ok {
		return nil
	}
	listPath := validate.Join(path, key)
	result := make([]T, 0, len(items))
	for index, item := range items {
		result = append(result, check(c, item, validate.Index(listPath, index)))
	}
	return result
}

func checkDependenciesField(c *validate.Checker, record map[string]any, path string) Dependencies {
	value, ok := c.Field(record, "dependencies", path)
	if !ok {
		return Dependencies{}
	}
	dependenciesPath := validate.Join(path, "dependencies")
	declared, ok := c.Record(value, dependenciesPath)
	if !ok {
		return Dependencies{}
	}
	c.RejectUnknown(declared, DependencyFields, dependenciesPath)

	dependencies := Dependencies{}
	dependencies.Capabilities = checkList(c, declared, "capabilities", dependenciesPath, checkCapabilityDependency)
	dependencies.Components = checkList(c, declared, "components", dependenciesPath, checkComponentDependency)
	dependencies.Framework, _ = c.OptionalString(declared, "framework", dependenciesPath)
	return dependencies
}

func checkCapabilityDependency(c *validate.Checker, value any, path string) CapabilityDependency {
	declared, ok := c.Record(value, path)
	if !ok {
		return CapabilityDependency{}
	}
	dependency := CapabilityDependency{}
	dependency.Name, _ = c.RequireString(declared, "name", path)
	dependency.Version, _ = c.OptionalString(declared, "version", path)
	dependency.Guarantees = checkGuarantees(c, declared, path)
	if optional, ok := c.Optional(declared, "optional"); ok {
		dependency.Optional, _ = c.Boolean(optional, validate.Join(path, "optional"))
	}
	return dependency
}

// KOSD-C-09 fixes a guarantee level as a boolean or an enumeration value.
func checkGuarantees(c *validate.Checker, record map[string]any, path string) map[string]any {
	declared, ok := c.OptionalRecord(record, "guarantees", path)
	if !ok {
		return nil
	}
	guaranteesPath := validate.Join(path, "guarantees")
	for _, key := range sortedKeys(declared) {
		switch declared[key].(type) {
		case bool, string:
		default:
			c.Add(validate.Join(guaranteesPath, key), validate.CodeInvalidType,
				"a guarantee level is a boolean or an enumeration value (KOSD-C-09)")
		}
	}
	return declared
}

func checkComponentDependency(c *validate.Checker, value any, path string) ComponentDependency {
	declared, ok := c.Record(value, path)
	if !ok {
		return ComponentDependency{}
	}
	dependency := ComponentDependency{}
	dependency.ID, _ = c.RequireString(declared, "id", path)
	dependency.Version, _ = c.OptionalString(declared, "version", path)
	return dependency
}

func checkExtensionPermissionsField(c *validate.Checker, record map[string]any, path string) ExtensionPermissions {
	value, ok := c.Field(record, "permissions", path)
	if !ok {
		return ExtensionPermissions{}
	}
	permissionsPath := validate.Join(path, "permissions")
	declared, ok := c.Record(value, permissionsPath)
	if !ok {
		return ExtensionPermissions{}
	}
	c.RejectUnknown(declared, ExtensionPermissionFields, permissionsPath)

	permissions := ExtensionPermissions{}
	permissions.Write = checkList(c, declared, "write", permissionsPath, checkAPIScope)
	permissions.Read = checkList(c, declared, "read", permissionsPath, checkReadScope)
	permissions.Roles, _ = c.OptionalStringArray(declared, "roles", permissionsPath)
	permissions.ObjectRoles = checkObjectRoles(c, declared, permissionsPath)
	return permissions
}

func checkAPIScope(c *validate.Checker, value any, path string) WriteScope {
	declared, ok := c.Record(value, path)
	if !ok {
		return WriteScope{}
	}
	c.RejectUnknown(declared, WriteScopeFields, path)

	scope := WriteScope{}
	scope.APIGroup, _ = c.RequireString(declared, "apiGroup", path)
	scope.Resources, _ = c.RequireStringArray(declared, "resources", path)
	scope.Verbs = checkAPIVerbs(c, declared, path)
	return scope
}

func checkReadScope(c *validate.Checker, value any, path string) ReadScope {
	declared, ok := c.Record(value, path)
	if !ok {
		return ReadScope{}
	}
	c.RejectUnknown(declared, ReadScopeFields, path)

	scope := ReadScope{}
	scope.APIGroup, _ = c.RequireString(declared, "apiGroup", path)
	scope.Resources, _ = c.RequireStringArray(declared, "resources", path)
	return scope
}

func checkAPIVerbs(c *validate.Checker, record map[string]any, path string) []string {
	verbs, ok := c.RequireStringArray(record, "verbs", path)
	if !ok {
		return nil
	}
	verbsPath := validate.Join(path, "verbs")
	accepted := make([]string, 0, len(verbs))
	for index, verb := range verbs {
		if !isWriteVerb(verb) {
			c.Add(validate.Index(verbsPath, index), validate.CodeInvalidValue,
				"unknown verb %q; expected one of %v", verb, extension.WriteVerbs)
			continue
		}
		accepted = append(accepted, verb)
	}
	return accepted
}

func checkObjectRoles(c *validate.Checker, record map[string]any, path string) map[string][]Role {
	declared, ok := c.OptionalRecord(record, "objectRoles", path)
	if !ok {
		return nil
	}
	rolesPath := validate.Join(path, "objectRoles")
	objectRoles := make(map[string][]Role, len(declared))
	for _, kind := range sortedKeys(declared) {
		entryPath := validate.Join(rolesPath, kind)
		names, ok := c.StringArray(declared[kind], entryPath)
		if !ok {
			continue
		}
		roles := make([]Role, 0, len(names))
		for _, name := range names {
			if !isKosRole(name) {
				c.Add(entryPath, validate.CodeInvalidValue,
					"unknown role %q; expected one of %v", name, KosRoles)
				continue
			}
			roles = append(roles, Role(name))
		}
		objectRoles[kind] = roles
	}
	return objectRoles
}

func checkStatusField(c *validate.Checker, record map[string]any, path string) StatusDeclaration {
	value, ok := c.Field(record, "status", path)
	if !ok {
		return StatusDeclaration{}
	}
	statusPath := validate.Join(path, "status")
	declared, ok := c.Record(value, statusPath)
	if !ok {
		return StatusDeclaration{}
	}
	c.RejectUnknown(declared, StatusFields, statusPath)

	status := StatusDeclaration{}
	status.Component = checkComponentStatus(c, declared, statusPath)
	status.ObjectClasses = checkObjectStatuses(c, declared, statusPath)
	return status
}

func checkComponentStatus(c *validate.Checker, record map[string]any, path string) *ComponentStatus {
	value, ok := c.Optional(record, "component")
	if !ok {
		return nil
	}
	componentPath := validate.Join(path, "component")
	declared, ok := c.Record(value, componentPath)
	if !ok {
		return nil
	}
	c.RejectUnknown(declared, ComponentStatusFields, componentPath)
	component := &ComponentStatus{}
	component.Conditions, _ = c.OptionalStringArray(declared, "conditions", componentPath)
	return component
}

func checkObjectStatuses(c *validate.Checker, record map[string]any, path string) map[string]ObjectStatus {
	value, ok := c.Optional(record, "objectClasses")
	if !ok {
		return nil
	}
	classesPath := validate.Join(path, "objectClasses")
	declared, ok := c.Record(value, classesPath)
	if !ok {
		return nil
	}
	statuses := make(map[string]ObjectStatus, len(declared))
	for _, kind := range sortedKeys(declared) {
		entryPath := validate.Join(classesPath, kind)
		entry, ok := c.Record(declared[kind], entryPath)
		if !ok {
			continue
		}
		c.RejectUnknown(entry, ObjectStatusFields, entryPath)
		status := ObjectStatus{}
		status.Writer, _ = c.RequireString(entry, "writer", entryPath)
		status.Conditions, _ = c.RequireStringArray(entry, "conditions", entryPath)
		statuses[kind] = status
	}
	return statuses
}

func checkRetentionField(c *validate.Checker, record map[string]any, path string) UninstallRetention {
	value, ok := c.Field(record, "uninstallRetention", path)
	if !ok {
		return UninstallRetention{}
	}
	retentionPath := validate.Join(path, "uninstallRetention")
	declared, ok := c.Record(value, retentionPath)
	if !ok {
		return UninstallRetention{}
	}
	c.RejectUnknown(declared, RetentionFields, retentionPath)

	retention := UninstallRetention{}
	retention.Contributions = checkRetentionActions(c, declared, retentionPath)
	retention.ExternalResources, _ = c.OptionalStringArray(declared, "externalResources", retentionPath)
	return retention
}

func checkRetentionActions(c *validate.Checker, record map[string]any, path string) map[string]RetentionAction {
	value, ok := c.Optional(record, "contributions")
	if !ok {
		return nil
	}
	contributionsPath := validate.Join(path, "contributions")
	declared, ok := c.Record(value, contributionsPath)
	if !ok {
		return nil
	}
	actions := make(map[string]RetentionAction, len(declared))
	for _, key := range sortedKeys(declared) {
		action, ok := declared[key].(string)
		if !ok || (RetentionAction(action) != Retain && RetentionAction(action) != Delete) {
			c.Add(validate.Join(contributionsPath, key), validate.CodeInvalidValue,
				"retention is either %q or %q (KOSD-C-18)", Retain, Delete)
			continue
		}
		actions[key] = RetentionAction(action)
	}
	return actions
}

func isWriteVerb(verb string) bool {
	for _, allowed := range extension.WriteVerbs {
		if allowed == verb {
			return true
		}
	}
	return false
}

func isKosRole(role string) bool {
	for _, allowed := range KosRoles {
		if allowed == role {
			return true
		}
	}
	return false
}

func sortedKeys(record map[string]any) []string {
	keys := make([]string, 0, len(record))
	for key := range record {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
