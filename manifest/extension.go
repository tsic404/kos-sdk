package manifest

import (
	"strings"

	"github.com/tsic404/kos-sdk/extension"
	"github.com/tsic404/kos-sdk/validate"
)

// ExtensionManifestFields is the declaration's closed top-level field set. The
// framework fixes it (KOSD-C-18); the SDK adds no field of its own.
var ExtensionManifestFields = []string{
	"identifier",
	"version",
	"frameworkVersion",
	"apiGroup",
	"contributions",
	"dependencies",
	"permissions",
	"configuration",
	"retention",
}

// ContributionClasses are the ten contribution classes of KOSD-C-18.
var ContributionClasses = []string{
	"objectClasses",
	"controllers",
	"capabilities",
	"pages",
	"settings",
	"catalogueSources",
	"discoveryAdapters",
	"diagnosticAdapters",
	"preferences",
	"apps",
}

// DependencyFields is the dependency block's closed field set.
var DependencyFields = []string{"capabilities", "components"}

// CapabilityRequirementFields is one capability dependency's closed field set.
var CapabilityRequirementFields = []string{"name", "versions"}

// ComponentRequirementFields is one component dependency's closed field set.
var ComponentRequirementFields = []string{"identifier", "versions"}

// CapabilityContributionFields is the capability contribution block's closed
// field set.
var CapabilityContributionFields = []string{"items", "storageClasses", "selfReported"}

// PermissionFields is one permission entry's closed field set.
var PermissionFields = []string{"contribution", "readScope", "requiredRole", "objectWriters"}

// ReadScopeFields is one read scope's closed field set.
var ReadScopeFields = []string{"groups", "kinds"}

// ConfigurationFields is the configuration block's closed field set.
var ConfigurationFields = []string{"schema", "statusWriters", "conditions"}

// RetentionFields is one retention entry's closed field set.
var RetentionFields = []string{"resource", "keep", "externalReference"}

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

// Contributions carries the ten contribution classes. A class the author did
// not declare is absent from the produced manifest.
type Contributions struct {
	ObjectClasses      []extension.ObjectClassDeclaration `json:"objectClasses,omitempty"`
	Controllers        []extension.ControllerDeclaration  `json:"controllers,omitempty"`
	Capabilities       *CapabilityContributions           `json:"capabilities,omitempty"`
	Pages              []map[string]any                   `json:"pages,omitempty"`
	Settings           []map[string]any                   `json:"settings,omitempty"`
	CatalogueSources   []map[string]any                   `json:"catalogueSources,omitempty"`
	DiscoveryAdapters  []map[string]any                   `json:"discoveryAdapters,omitempty"`
	DiagnosticAdapters []map[string]any                   `json:"diagnosticAdapters,omitempty"`
	Preferences        []map[string]any                   `json:"preferences,omitempty"`
	Apps               []AppManifest                      `json:"apps,omitempty"`
}

// CapabilityContributions is the capability contribution class: the capability
// entries, the storage classes the provider supplies and its own conformance
// report (KOSD-C-18, KOSD-C-47).
type CapabilityContributions struct {
	Items          []map[string]any `json:"items,omitempty"`
	StorageClasses []string         `json:"storageClasses,omitempty"`
	SelfReported   []map[string]any `json:"selfReported,omitempty"`
}

// Dependencies are the capability, component and framework edges of the
// declaration (KOSD-C-18).
type Dependencies struct {
	Capabilities []CapabilityRequirement `json:"capabilities,omitempty"`
	Components   []ComponentRequirement  `json:"components,omitempty"`
}

// CapabilityRequirement is one capability edge; an empty Versions admits every
// version.
type CapabilityRequirement struct {
	Name     string `json:"name"`
	Versions string `json:"versions,omitempty"`
}

// ComponentRequirement requires another installed component.
type ComponentRequirement struct {
	Identifier string `json:"identifier"`
	Versions   string `json:"versions,omitempty"`
}

// Permission is one declared permission: the contribution it belongs to, what
// that contribution reads, the control-plane role it needs and the roles that
// may write the extension's objects (KOSD-C-18).
type ExtensionPermission struct {
	Contribution  string    `json:"contribution"`
	ReadScope     ReadScope `json:"readScope,omitempty"`
	RequiredRole  string    `json:"requiredRole,omitempty"`
	ObjectWriters []Role    `json:"objectWriters,omitempty"`
}

// ReadScope is the read side of a declared permission: API groups and kinds.
// An empty group is the core group, so reading Event or Node needs no invented
// group name.
type ReadScope struct {
	Groups []string              `json:"groups,omitempty"`
	Kinds  []extension.GroupKind `json:"kinds,omitempty"`
}

// Configuration is the declared component configuration structure and the
// status interface its objects expose (KOSD-C-18).
type Configuration struct {
	Schema        map[string]any `json:"schema,omitempty"`
	StatusWriters []Role         `json:"statusWriters,omitempty"`
	Conditions    []string       `json:"conditions,omitempty"`
}

// Retention is the declared fate of one contribution's data when its component
// is uninstalled (KOSD-C-18, KOSD-C-29).
type Retention struct {
	Resource          string `json:"resource"`
	Keep              bool   `json:"keep"`
	ExternalReference string `json:"externalReference,omitempty"`
}

// ExtensionManifest is the extension declaration of KOSD-C-18.
type ExtensionManifest struct {
	Identifier       string                `json:"identifier"`
	Version          string                `json:"version"`
	FrameworkVersion string                `json:"frameworkVersion"`
	APIGroup         string                `json:"apiGroup,omitempty"`
	Contributions    Contributions         `json:"contributions"`
	Dependencies     Dependencies          `json:"dependencies"`
	Permissions      []ExtensionPermission `json:"permissions"`
	Configuration    Configuration         `json:"configuration"`
	Retention        []Retention           `json:"retention"`
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
	manifest.Identifier, _ = c.RequireString(record, "identifier", path)
	manifest.Version, _ = c.RequireString(record, "version", path)
	manifest.FrameworkVersion, _ = c.RequireString(record, "frameworkVersion", path)
	manifest.APIGroup = checkAPIGroup(c, record, path)
	manifest.Contributions = checkContributionsField(c, record, path)
	manifest.Dependencies = checkDependenciesField(c, record, path)
	manifest.Permissions = checkPermissionsField(c, record, path)
	manifest.Configuration = checkConfigurationField(c, record, path)
	manifest.Retention = checkRetentionField(c, record, path)
	checkClassGroups(c, manifest, path)
	return manifest
}

// checkAPIGroup reads the extension's own API group. Object classes belong to
// it, so it stays optional only for declarations that contribute none (C-19).
func checkAPIGroup(c *validate.Checker, record map[string]any, path string) string {
	raw, present := c.Optional(record, "apiGroup")
	if !present {
		return ""
	}
	apiGroup, ok := raw.(string)
	if !ok || strings.TrimSpace(apiGroup) != apiGroup {
		c.Add(validate.Join(path, "apiGroup"), validate.CodeInvalidType,
			"expected a string with no surrounding whitespace")
		return ""
	}
	return apiGroup
}

// checkClassGroups holds every declared class to the declaration's own API
// group; the kernel refuses a class outside it (KOSD-C-19).
func checkClassGroups(c *validate.Checker, manifest ExtensionManifest, path string) {
	classesPath := validate.Join(validate.Join(path, "contributions"), "objectClasses")
	for index, class := range manifest.Contributions.ObjectClasses {
		entryPath := validate.Join(validate.Index(classesPath, index), "group")
		switch {
		case class.Group == "":
			continue
		case manifest.APIGroup == "":
			c.Add(entryPath, validate.CodeMissingField,
				"an object class needs the declaration's apiGroup (KOSD-C-19)")
		case class.Group != manifest.APIGroup:
			c.Add(entryPath, validate.CodeInvalidValue,
				"group %q must equal the declaration's apiGroup %q (KOSD-C-19)", class.Group, manifest.APIGroup)
		}
	}
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
	contributions.Capabilities = checkCapabilities(c, declared, contributionsPath)
	contributions.Pages = checkPayloadList(c, declared, "pages", contributionsPath)
	contributions.Settings = checkPayloadList(c, declared, "settings", contributionsPath)
	contributions.CatalogueSources = checkPayloadList(c, declared, "catalogueSources", contributionsPath)
	contributions.DiscoveryAdapters = checkPayloadList(c, declared, "discoveryAdapters", contributionsPath)
	contributions.DiagnosticAdapters = checkPayloadList(c, declared, "diagnosticAdapters", contributionsPath)
	contributions.Preferences = checkPayloadList(c, declared, "preferences", contributionsPath)
	contributions.Apps = checkList(c, declared, "apps", contributionsPath, checkApp)
	return contributions
}

func checkCapabilities(c *validate.Checker, record map[string]any, path string) *CapabilityContributions {
	value, ok := c.Optional(record, "capabilities")
	if !ok {
		return nil
	}
	capabilitiesPath := validate.Join(path, "capabilities")
	declared, ok := c.Record(value, capabilitiesPath)
	if !ok {
		return nil
	}
	c.RejectUnknown(declared, CapabilityContributionFields, capabilitiesPath)

	capabilities := &CapabilityContributions{}
	capabilities.Items = payloadEntries(c, declared, "items", capabilitiesPath)
	capabilities.StorageClasses, _ = c.OptionalStringArray(declared, "storageClasses", capabilitiesPath)
	capabilities.SelfReported = payloadEntries(c, declared, "selfReported", capabilitiesPath)
	return capabilities
}

// checkPayloadList accepts a contribution class whose entries belong to their
// own contract, not to this manifest; each entry stays an object.
func checkPayloadList(c *validate.Checker, record map[string]any, key, path string) []map[string]any {
	items, ok := c.OptionalArray(record, key, path)
	if !ok {
		return nil
	}
	listPath := validate.Join(path, key)
	payloads := make([]map[string]any, 0, len(items))
	for index, item := range items {
		payload, ok := c.Record(item, validate.Index(listPath, index))
		if !ok {
			continue
		}
		payloads = append(payloads, payload)
	}
	return payloads
}

// payloadEntries reads an object field whose entries are author payloads.
func payloadEntries(c *validate.Checker, record map[string]any, key, path string) []map[string]any {
	items, ok := c.OptionalArray(record, key, path)
	if !ok {
		return nil
	}
	listPath := validate.Join(path, key)
	payloads := make([]map[string]any, 0, len(items))
	for index, item := range items {
		payload, ok := c.Record(item, validate.Index(listPath, index))
		if !ok {
			continue
		}
		payloads = append(payloads, payload)
	}
	return payloads
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
	dependencies.Capabilities = checkList(c, declared, "capabilities", dependenciesPath, checkCapabilityRequirement)
	dependencies.Components = checkList(c, declared, "components", dependenciesPath, checkComponentRequirement)
	return dependencies
}

func checkCapabilityRequirement(c *validate.Checker, value any, path string) CapabilityRequirement {
	declared, ok := c.Record(value, path)
	if !ok {
		return CapabilityRequirement{}
	}
	c.RejectUnknown(declared, CapabilityRequirementFields, path)

	requirement := CapabilityRequirement{}
	requirement.Name, _ = c.RequireString(declared, "name", path)
	requirement.Versions, _ = c.OptionalString(declared, "versions", path)
	return requirement
}

func checkComponentRequirement(c *validate.Checker, value any, path string) ComponentRequirement {
	declared, ok := c.Record(value, path)
	if !ok {
		return ComponentRequirement{}
	}
	c.RejectUnknown(declared, ComponentRequirementFields, path)

	requirement := ComponentRequirement{}
	requirement.Identifier, _ = c.RequireString(declared, "identifier", path)
	requirement.Versions, _ = c.OptionalString(declared, "versions", path)
	return requirement
}

func checkPermissionsField(c *validate.Checker, record map[string]any, path string) []ExtensionPermission {
	value, ok := c.Field(record, "permissions", path)
	if !ok {
		return nil
	}
	permissionsPath := validate.Join(path, "permissions")
	items, ok := c.Array(value, permissionsPath)
	if !ok {
		return nil
	}
	permissions := make([]ExtensionPermission, 0, len(items))
	for index, item := range items {
		permissions = append(permissions, checkPermission(c, item, validate.Index(permissionsPath, index)))
	}
	return permissions
}

func checkPermission(c *validate.Checker, value any, path string) ExtensionPermission {
	declared, ok := c.Record(value, path)
	if !ok {
		return ExtensionPermission{}
	}
	c.RejectUnknown(declared, PermissionFields, path)

	permission := ExtensionPermission{}
	permission.Contribution, _ = c.RequireString(declared, "contribution", path)
	permission.ReadScope = checkReadScope(c, declared, path)
	permission.RequiredRole = checkOptionalRole(c, declared, "requiredRole", path)
	permission.ObjectWriters = checkOptionalRoles(c, declared, "objectWriters", path)
	return permission
}

// checkReadScope accepts the kernel's scope shape: groups and kinds, each with
// an optional group whose empty value names the core group.
func checkReadScope(c *validate.Checker, record map[string]any, path string) ReadScope {
	value, ok := c.Optional(record, "readScope")
	if !ok {
		return ReadScope{}
	}
	scopePath := validate.Join(path, "readScope")
	declared, ok := c.Record(value, scopePath)
	if !ok {
		return ReadScope{}
	}
	c.RejectUnknown(declared, ReadScopeFields, scopePath)

	scope := ReadScope{}
	scope.Groups = checkGroupList(c, declared, "groups", scopePath)
	if items, ok := c.OptionalArray(declared, "kinds", scopePath); ok {
		kindsPath := validate.Join(scopePath, "kinds")
		scope.Kinds = make([]extension.GroupKind, 0, len(items))
		for index, item := range items {
			scope.Kinds = append(scope.Kinds, extension.CheckGroupKind(c, item, validate.Index(kindsPath, index)))
		}
	}
	return scope
}

// checkGroupList reads API groups that may name the core group as the empty
// string.
func checkGroupList(c *validate.Checker, record map[string]any, key, path string) []string {
	items, ok := c.OptionalArray(record, key, path)
	if !ok {
		return nil
	}
	listPath := validate.Join(path, key)
	groups := make([]string, 0, len(items))
	for index, item := range items {
		group, ok := item.(string)
		if !ok {
			c.Add(validate.Index(listPath, index), validate.CodeInvalidType, "expected a string")
			continue
		}
		groups = append(groups, group)
	}
	return groups
}

func checkOptionalRole(c *validate.Checker, record map[string]any, key, path string) string {
	role, ok := c.OptionalString(record, key, path)
	if !ok {
		return ""
	}
	if !isKosRole(role) {
		c.Add(validate.Join(path, key), validate.CodeInvalidValue,
			"unknown role %q; expected one of %v", role, KosRoles)
		return ""
	}
	return role
}

func checkOptionalRoles(c *validate.Checker, record map[string]any, key, path string) []Role {
	names, ok := c.OptionalStringArray(record, key, path)
	if !ok {
		return nil
	}
	rolesPath := validate.Join(path, key)
	roles := make([]Role, 0, len(names))
	for index, name := range names {
		if !isKosRole(name) {
			c.Add(validate.Index(rolesPath, index), validate.CodeInvalidValue,
				"unknown role %q; expected one of %v", name, KosRoles)
			continue
		}
		roles = append(roles, Role(name))
	}
	return roles
}

func checkConfigurationField(c *validate.Checker, record map[string]any, path string) Configuration {
	value, ok := c.Field(record, "configuration", path)
	if !ok {
		return Configuration{}
	}
	configurationPath := validate.Join(path, "configuration")
	declared, ok := c.Record(value, configurationPath)
	if !ok {
		return Configuration{}
	}
	c.RejectUnknown(declared, ConfigurationFields, configurationPath)

	configuration := Configuration{}
	configuration.Schema, _ = c.OptionalRecord(declared, "schema", configurationPath)
	configuration.StatusWriters = checkOptionalRoles(c, declared, "statusWriters", configurationPath)
	configuration.Conditions, _ = c.OptionalStringArray(declared, "conditions", configurationPath)
	return configuration
}

func checkRetentionField(c *validate.Checker, record map[string]any, path string) []Retention {
	value, ok := c.Field(record, "retention", path)
	if !ok {
		return nil
	}
	retentionPath := validate.Join(path, "retention")
	items, ok := c.Array(value, retentionPath)
	if !ok {
		return nil
	}
	retention := make([]Retention, 0, len(items))
	for index, item := range items {
		retention = append(retention, checkRetention(c, item, validate.Index(retentionPath, index)))
	}
	return retention
}

func checkRetention(c *validate.Checker, value any, path string) Retention {
	declared, ok := c.Record(value, path)
	if !ok {
		return Retention{}
	}
	c.RejectUnknown(declared, RetentionFields, path)

	entry := Retention{}
	entry.Resource, _ = c.RequireString(declared, "resource", path)
	entry.Keep, _ = c.RequireBoolean(declared, "keep", path)
	entry.ExternalReference, _ = c.OptionalString(declared, "externalReference", path)
	return entry
}

func isKosRole(role string) bool {
	for _, allowed := range KosRoles {
		if allowed == role {
			return true
		}
	}
	return false
}
