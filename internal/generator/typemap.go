package generator

import (
	"fmt"
	"sort"
	"strings"
)

// GoTypeKind classifies a generated Go type.
type GoTypeKind int

const (
	GoTypeStruct GoTypeKind = iota
	GoTypeEnum
	GoTypeAlias
	GoTypeMap
)

// GoType represents a Go type to be generated.
type GoType struct {
	Name       string
	Doc        string
	SchemaFile string
	SchemaPath string // JSON pointer within the file
	Kind       GoTypeKind
	Fields     []GoField
	EnumValues []GoEnumVal
	AliasOf    GoTypeRef
	OutputFile string
	EmbedMeta  bool // Whether to embed FileMetadata
}

// GoField represents a field in a Go struct.
type GoField struct {
	Name     string
	JSONName string
	Doc      string
	Type     GoTypeRef
	Required bool
}

// GoEnumVal represents a single enum constant.
type GoEnumVal struct {
	GoName string
	Value  string
}

// GoTypeRef represents a reference to a Go type, with modifiers.
type GoTypeRef struct {
	Builtin  string // "string", "int", "bool", "float64", "any"
	Named    string // reference to a GoType by name
	Pointer  bool
	Slice    bool
	Map      bool
	Element  *GoTypeRef // for slices
	MapKey   *GoTypeRef // for maps (usually string)
	MapValue *GoTypeRef // for maps
}

// IsAny returns true if this type ref represents any/interface{}.
func (r GoTypeRef) IsAny() bool {
	return r.Builtin == "any"
}

// String returns a human-readable representation.
func (r GoTypeRef) String() string {
	var s string
	switch {
	case r.Map:
		key := "string"
		if r.MapKey != nil {
			key = r.MapKey.String()
		}
		val := "any"
		if r.MapValue != nil {
			val = r.MapValue.String()
		}
		s = "map[" + key + "]" + val
	case r.Slice:
		elem := "any"
		if r.Element != nil {
			elem = r.Element.String()
		}
		s = "[]" + elem
	case r.Builtin != "":
		s = r.Builtin
	case r.Named != "":
		s = r.Named
	default:
		s = "any"
	}
	if r.Pointer {
		s = "*" + s
	}
	return s
}

// propInfo tracks a property schema along with the file it originates from.
type propInfo struct {
	schema      *Schema
	contextFile string
}

// TypeMapper converts a set of JSON Schema files into Go types.
type TypeMapper struct {
	registry *SchemaRegistry
	types    map[string]*GoType // by Go type name
	seen     map[string]string  // (file + "#" + pointer) → Go type name

	// entryPoints tracks schemas that are entry points to name
	// their root types using override names.
	entryPoints map[string]string // schema relPath → Go type name
}

// NewTypeMapper creates a TypeMapper backed by the given registry.
func NewTypeMapper(registry *SchemaRegistry) *TypeMapper {
	return &TypeMapper{
		registry:    registry,
		types:       make(map[string]*GoType),
		seen:        make(map[string]string),
		entryPoints: make(map[string]string),
	}
}

// Types returns all generated types sorted by name.
func (m *TypeMapper) Types() []*GoType {
	var result []*GoType
	for _, t := range m.types {
		result = append(result, t)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// TypesByName returns the map of types by name.
func (m *TypeMapper) TypesByName() map[string]*GoType {
	return m.types
}

// RegisterEntryPoint registers an entry point schema with a specific Go
// type name for the root type.
func (m *TypeMapper) RegisterEntryPoint(schemaRelPath, goTypeName string) {
	m.entryPoints[schemaRelPath] = goTypeName
}

// ProcessEntryPoint loads and processes a schema file, creating Go types
// for its root and all referenced definitions.
func (m *TypeMapper) ProcessEntryPoint(schemaRelPath string) error {
	schema, err := m.registry.LoadSchema(schemaRelPath)
	if err != nil {
		return err
	}

	rootName := ""
	if name, ok := m.entryPoints[schemaRelPath]; ok {
		rootName = name
	}

	_, err = m.processSchema(schema, schemaRelPath, "", rootName, true)
	return err
}

// processSchema recursively processes a schema and returns the Go type
// reference for it.
func (m *TypeMapper) processSchema(
	schema *Schema,
	contextFile string,
	jsonPointer string,
	suggestedName string,
	isEntryPoint bool,
) (GoTypeRef, error) {
	if schema == nil {
		return GoTypeRef{Builtin: "any"}, nil
	}

	// Handle boolean schemas.
	if schema.IsBooleanSchema() {
		return GoTypeRef{Builtin: "any"}, nil
	}

	// Handle $ref — always delegate to processRef which tracks context properly.
	if schema.Ref != "" {
		return m.processRef(schema.Ref, contextFile, schema.Description)
	}

	// Check dedup cache.
	cacheKey := contextFile + "#" + jsonPointer
	if jsonPointer != "" {
		if name, ok := m.seen[cacheKey]; ok {
			return GoTypeRef{Named: name}, nil
		}
	}

	// Determine the type.
	typ := schema.Type.Single()
	types := schema.Type.Values()

	// Handle multi-type (e.g. ["string", "null"]).
	if len(types) > 1 {
		return m.handleMultiType(types, schema, contextFile, jsonPointer, suggestedName)
	}

	// Handle enum with a string type → generate enum type.
	if len(schema.Enum) > 0 && typ == "string" && suggestedName != "" {
		return m.createEnumType(schema, contextFile, jsonPointer, suggestedName)
	}

	// Handle anyOf/oneOf that are just type unions (e.g. string | boolean → any).
	if typ == "" && schema.AnyOf != nil && !schema.HasProperties() {
		return m.processAnyOf(schema, contextFile, jsonPointer, suggestedName)
	}
	if typ == "" && schema.OneOf != nil && !schema.HasProperties() {
		return m.processOneOf(schema, contextFile, jsonPointer, suggestedName)
	}

	switch typ {
	case "object":
		return m.processObject(schema, contextFile, jsonPointer, suggestedName, isEntryPoint)
	case "array":
		return m.processArray(schema, contextFile, jsonPointer, suggestedName, isEntryPoint)
	case "string":
		return GoTypeRef{Builtin: "string"}, nil
	case "integer":
		return GoTypeRef{Builtin: "int"}, nil
	case "number":
		return GoTypeRef{Builtin: "float64"}, nil
	case "boolean":
		return GoTypeRef{Builtin: "bool"}, nil
	case "null":
		return GoTypeRef{Builtin: "any"}, nil
	}

	// No explicit type — check for properties (implicit object).
	if schema.HasProperties() {
		return m.processObject(schema, contextFile, jsonPointer, suggestedName, isEntryPoint)
	}

	// allOf at root level with no type and no properties.
	if schema.AllOf != nil {
		return m.processObject(schema, contextFile, jsonPointer, suggestedName, isEntryPoint)
	}

	return GoTypeRef{Builtin: "any"}, nil
}

// processRef resolves a $ref and processes the target schema.
func (m *TypeMapper) processRef(ref, contextFile, descOverride string) (GoTypeRef, error) {
	resolved, targetFile, err := m.registry.ResolveRef(ref, contextFile)
	if err != nil {
		return GoTypeRef{}, err
	}

	// Self-reference: # means the root of the current file.
	if ref == "#" {
		if resolved.Type.Single() == "array" && resolved.Items != nil {
			return m.processSchema(resolved, targetFile, "", "", false)
		}
	}

	_, fragment := splitRef(ref)
	pointer := fragment

	// Check if already processed.
	cacheKey := targetFile + "#" + pointer
	if name, ok := m.seen[cacheKey]; ok {
		return GoTypeRef{Named: name}, nil
	}

	// Derive a name from the ref fragment.
	defName := ""
	if fragment != "" {
		parts := strings.Split(strings.TrimPrefix(fragment, "/"), "/")
		// Use the last meaningful segment for naming.
		for i := len(parts) - 1; i >= 0; i-- {
			seg := parts[i]
			if seg != "definitions" && seg != "$defs" && seg != "properties" && seg != "items" {
				defName = seg
				break
			}
		}
	}

	suggestedName := ToGoName(defName)

	// Apply description override from the referring schema.
	if descOverride != "" && resolved.Description == "" {
		copy := *resolved
		copy.Description = descOverride
		resolved = &copy
	}

	// Process the resolved schema using the TARGET file as context.
	// This ensures that any nested $refs within it resolve correctly.
	return m.processSchema(resolved, targetFile, pointer, suggestedName, false)
}

// processObject creates a struct type from an object schema.
func (m *TypeMapper) processObject(
	schema *Schema,
	contextFile string,
	jsonPointer string,
	suggestedName string,
	isEntryPoint bool,
) (GoTypeRef, error) {
	// Handle map types (additionalProperties with schema, no explicit properties).
	if !schema.HasProperties() && schema.AdditionalProperties != nil && schema.AdditionalProperties.Schema != nil {
		return m.processMapType(schema, contextFile, jsonPointer, suggestedName)
	}
	// Bare object with no properties and additionalProperties not false.
	if !schema.HasProperties() && schema.AllOf == nil {
		if schema.AdditionalProperties == nil || (schema.AdditionalProperties.Bool != nil && *schema.AdditionalProperties.Bool) {
			return GoTypeRef{
				Map:      true,
				MapKey:   &GoTypeRef{Builtin: "string"},
				MapValue: &GoTypeRef{Builtin: "any"},
			}, nil
		}
	}

	if suggestedName == "" {
		suggestedName = "Object"
	}

	name := m.uniqueName(suggestedName)

	// Register early to handle recursive references.
	cacheKey := contextFile + "#" + jsonPointer
	if jsonPointer != "" {
		m.seen[cacheKey] = name
	}

	goType := &GoType{
		Name:       name,
		Doc:        cleanDoc(schema.Description),
		SchemaFile: contextFile,
		SchemaPath: jsonPointer,
		Kind:       GoTypeStruct,
		EmbedMeta:  isEntryPoint,
	}

	// Collect all properties from the schema tree (properties, allOf, if/then/else).
	allProps, allRequired, err := m.collectProperties(schema, contextFile)
	if err != nil {
		return GoTypeRef{}, fmt.Errorf("processing object %s: %w", name, err)
	}

	// Sort properties alphabetically by JSON name for deterministic output.
	propNames := make([]string, 0, len(allProps))
	for k := range allProps {
		propNames = append(propNames, k)
	}
	sort.Strings(propNames)

	for _, propName := range propNames {
		pi := allProps[propName]
		fieldName := ToGoName(propName)

		// Use the property's original context file for resolving any nested $refs.
		fieldRef, err := m.processSchema(
			pi.schema,
			pi.contextFile,
			jsonPointer+"/properties/"+propName,
			name+fieldName,
			false,
		)
		if err != nil {
			return GoTypeRef{}, fmt.Errorf("processing field %s.%s: %w", name, propName, err)
		}

		isRequired := allRequired[propName]

		// Use pointer for optional boolean fields.
		if !isRequired && fieldRef.Builtin == "bool" {
			fieldRef.Pointer = true
		}

		goType.Fields = append(goType.Fields, GoField{
			Name:     fieldName,
			JSONName: propName,
			Doc:      cleanDoc(pi.schema.Description),
			Type:     fieldRef,
			Required: isRequired,
		})
	}

	m.types[name] = goType
	return GoTypeRef{Named: name}, nil
}

// collectProperties gathers all properties from a schema, including those
// defined via allOf, if/then/else merging. Each property tracks its source
// file for correct $ref resolution.
func (m *TypeMapper) collectProperties(schema *Schema, contextFile string) (
	props map[string]propInfo,
	required map[string]bool,
	err error,
) {
	props = make(map[string]propInfo)
	required = make(map[string]bool)

	// Direct properties.
	for k, v := range schema.Properties {
		props[k] = propInfo{schema: v, contextFile: contextFile}
	}
	for _, r := range schema.Required {
		required[r] = true
	}

	// Merge allOf.
	for _, sub := range schema.AllOf {
		m.mergeConditionalProps(sub, contextFile, props, required)
	}

	// Merge if/then/else at root level.
	m.mergeConditionalProps(schema, contextFile, props, required)

	return props, required, nil
}

// mergeConditionalProps extracts properties from if/then/else and allOf
// structures and merges them into the props map.
func (m *TypeMapper) mergeConditionalProps(
	schema *Schema,
	contextFile string,
	props map[string]propInfo,
	required map[string]bool,
) {
	if schema == nil {
		return
	}

	// Direct properties.
	for k, v := range schema.Properties {
		if _, exists := props[k]; !exists {
			props[k] = propInfo{schema: v, contextFile: contextFile}
		}
	}

	// Then branch.
	if schema.Then != nil {
		for k, v := range schema.Then.Properties {
			if _, exists := props[k]; !exists {
				props[k] = propInfo{schema: v, contextFile: contextFile}
			}
		}
	}

	// Else branch.
	if schema.Else != nil {
		for k, v := range schema.Else.Properties {
			if _, exists := props[k]; !exists {
				props[k] = propInfo{schema: v, contextFile: contextFile}
			}
		}
	}

	// OneOf — merge all branch properties as optional.
	for _, oneOfSchema := range schema.OneOf {
		for k, v := range oneOfSchema.Properties {
			if _, exists := props[k]; !exists {
				props[k] = propInfo{schema: v, contextFile: contextFile}
			}
		}
	}

	// Recurse into nested allOf.
	for _, sub := range schema.AllOf {
		m.mergeConditionalProps(sub, contextFile, props, required)
	}
}

// processArray handles array schemas.
func (m *TypeMapper) processArray(
	schema *Schema,
	contextFile string,
	jsonPointer string,
	suggestedName string,
	isEntryPoint bool,
) (GoTypeRef, error) {
	if schema.Items == nil {
		return GoTypeRef{
			Slice:   true,
			Element: &GoTypeRef{Builtin: "any"},
		}, nil
	}

	// Self-reference: items.$ref == "#" means recursive array.
	if schema.Items.Ref == "#" {
		rootSchema, err := m.registry.LoadSchema(contextFile)
		if err != nil {
			return GoTypeRef{}, err
		}
		if rootSchema.Items != nil && rootSchema.Items != schema.Items {
			elemRef, err := m.processSchema(rootSchema.Items, contextFile, "/items", suggestedName, false)
			if err != nil {
				return GoTypeRef{}, err
			}
			return GoTypeRef{Slice: true, Element: &elemRef}, nil
		}
		if suggestedName != "" {
			return GoTypeRef{Slice: true, Element: &GoTypeRef{Named: suggestedName}}, nil
		}
	}

	// Element name.
	elemName := ""
	if suggestedName != "" {
		elemName = singularize(suggestedName)
	}

	elemRef, err := m.processSchema(schema.Items, contextFile, jsonPointer+"/items", elemName, false)
	if err != nil {
		return GoTypeRef{}, err
	}

	// For entry-point arrays, the element type gets EmbedMeta.
	if isEntryPoint && elemRef.Named != "" {
		if t, ok := m.types[elemRef.Named]; ok {
			t.EmbedMeta = true
		}
	}

	return GoTypeRef{Slice: true, Element: &elemRef}, nil
}

// processMapType handles object schemas that represent maps.
func (m *TypeMapper) processMapType(
	schema *Schema,
	contextFile string,
	jsonPointer string,
	suggestedName string,
) (GoTypeRef, error) {
	valueRef, err := m.processSchema(
		schema.AdditionalProperties.Schema,
		contextFile,
		jsonPointer+"/additionalProperties",
		suggestedName+"Value",
		false,
	)
	if err != nil {
		return GoTypeRef{}, err
	}
	return GoTypeRef{
		Map:      true,
		MapKey:   &GoTypeRef{Builtin: "string"},
		MapValue: &valueRef,
	}, nil
}

// processAnyOf handles anyOf schemas (type unions).
func (m *TypeMapper) processAnyOf(
	schema *Schema,
	contextFile string,
	jsonPointer string,
	suggestedName string,
) (GoTypeRef, error) {
	allSimple := true
	for _, alt := range schema.AnyOf {
		if alt.HasProperties() || alt.Items != nil {
			allSimple = false
			break
		}
	}
	if allSimple {
		return GoTypeRef{Builtin: "any"}, nil
	}
	for _, alt := range schema.AnyOf {
		if alt.Type.Single() != "null" {
			return m.processSchema(alt, contextFile, jsonPointer, suggestedName, false)
		}
	}
	return GoTypeRef{Builtin: "any"}, nil
}

// processOneOf handles oneOf schemas.
func (m *TypeMapper) processOneOf(
	schema *Schema,
	contextFile string,
	jsonPointer string,
	suggestedName string,
) (GoTypeRef, error) {
	allSimple := true
	for _, alt := range schema.OneOf {
		if alt.HasProperties() || alt.Items != nil {
			allSimple = false
			break
		}
	}
	if allSimple {
		return GoTypeRef{Builtin: "any"}, nil
	}
	return GoTypeRef{Builtin: "any"}, nil
}

// handleMultiType handles schemas with multiple types like ["string", "null"].
func (m *TypeMapper) handleMultiType(
	types []string,
	schema *Schema,
	contextFile string,
	jsonPointer string,
	suggestedName string,
) (GoTypeRef, error) {
	var nonNull []string
	for _, t := range types {
		if t != "null" {
			nonNull = append(nonNull, t)
		}
	}
	hasNull := len(nonNull) < len(types)

	if len(nonNull) == 1 {
		ref, err := m.processSchema(
			&Schema{Type: SchemaType{values: []string{nonNull[0]}}, Properties: schema.Properties},
			contextFile, jsonPointer, suggestedName, false,
		)
		if err != nil {
			return GoTypeRef{}, err
		}
		if hasNull {
			ref.Pointer = true
		}
		return ref, nil
	}

	return GoTypeRef{Builtin: "any"}, nil
}

// createEnumType creates a named string type with enum constants.
func (m *TypeMapper) createEnumType(
	schema *Schema,
	contextFile string,
	jsonPointer string,
	suggestedName string,
) (GoTypeRef, error) {
	name := m.uniqueName(suggestedName)

	cacheKey := contextFile + "#" + jsonPointer
	if jsonPointer != "" {
		m.seen[cacheKey] = name
	}

	goType := &GoType{
		Name:       name,
		Doc:        cleanDoc(schema.Description),
		SchemaFile: contextFile,
		SchemaPath: jsonPointer,
		Kind:       GoTypeEnum,
	}

	values := schema.EnumStrings()
	for _, v := range values {
		goName := enumConstName(name, v)
		goType.EnumValues = append(goType.EnumValues, GoEnumVal{
			GoName: goName,
			Value:  v,
		})
	}

	m.types[name] = goType
	return GoTypeRef{Named: name}, nil
}

// uniqueName returns a unique Go type name, appending a number if needed.
func (m *TypeMapper) uniqueName(name string) string {
	if name == "" {
		name = "Type"
	}
	if _, exists := m.types[name]; !exists {
		return name
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s%d", name, i)
		if _, exists := m.types[candidate]; !exists {
			return candidate
		}
	}
}

// cleanDoc cleans up a schema description for use as a Go doc comment.
func cleanDoc(s string) string {
	s = strings.TrimSpace(s)
	return s
}

// enumConstName generates a valid Go constant name for an enum value.
func enumConstName(typeName, value string) string {
	// Handle special characters.
	cleaned := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-', r == '.':
			return r
		case r == '*':
			return -1 // Drop.
		default:
			return '_'
		}
	}, value)

	if cleaned == "" {
		return typeName + "Any"
	}
	return typeName + ToGoName(cleaned)
}

// singularize attempts to produce a singular form of a plural name.
func singularize(s string) string {
	if strings.HasSuffix(s, "ies") && len(s) > 3 {
		return s[:len(s)-3] + "y"
	}
	if strings.HasSuffix(s, "ses") {
		return s[:len(s)-2]
	}
	if strings.HasSuffix(s, "s") && !strings.HasSuffix(s, "ss") && !strings.HasSuffix(s, "us") {
		return s[:len(s)-1]
	}
	return s
}
