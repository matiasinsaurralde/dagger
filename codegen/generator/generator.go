package generator

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/dagger/dagger/codegen/introspection"
	"github.com/dagger/dagger/router"
	"github.com/iancoleman/strcase"
)

var (
	ErrUnknownSDKLang      = errors.New("unknown sdk language")
	FormatDeprecationRegex = regexp.MustCompile("`[a-zA-Z0-9_]+`")
)

type SDKLang string

const (
	SDKLangGo     SDKLang = "go"
	SDKLangNodeJS SDKLang = "nodejs"
	SDKLangPython SDKLang = "python"

	QueryStructName       = "Query"
	QueryStructClientName = "Client"
)

type Config struct {
	Lang SDKLang
	// Package is the target package that is generated.
	// Not used for the SDKLangNodeJS.
	Package string
}

// CustomScalar registers custom Dagger type.
// TODO: This may done it dynamically later instead of a static
// map.
var CustomScalar = map[string]string{
	"ContainerID":      "Container",
	"FileID":           "File",
	"DirectoryID":      "Directory",
	"SecretID":         "Secret",
	"SocketID":         "Socket",
	"CacheID":          "CacheVolume",
	"ProjectID":        "Project",
	"ProjectCommandID": "ProjectCommand",
}

type Generator interface {
	Generate(ctx context.Context, schema *introspection.Schema) ([]byte, error)
	SetConfig(cfg *Config)
	LoadTemplates() (*template.Template, error)

	FormatName(string) string
	FormatEnum(string) string
	SortEnumFields([]introspection.EnumValue) []introspection.EnumValue
	IsEnum(*introspection.Type) bool
	FormatDeprecation(string) string
	Comment(string) string

	// Taken from common func:
	FormatKindList(string) string
	FormatKindScalarString(string) string
	FormatKindScalarInt(string) string
	FormatKindScalarFloat(string) string
	FormatKindScalarBoolean(string) string
	FormatKindScalarDefault(string, string, bool) string
	FormatKindObject(string, string) string
	FormatKindInputObject(string, string) string
	FormatKindEnum(string, string) string

	FormatReturnType(introspection.Field) string
	FormatInputType(*introspection.TypeRef) string
	FormatOutputType(*introspection.TypeRef) string

	ConvertID(introspection.Field) bool

	IsSelfChainable(introspection.Type) bool
}

// BaseGenerator provides default implementations for common methods
// It also holds the generator configuration data structure
type BaseGenerator struct {
	Config *Config
}

func (b *BaseGenerator) SetConfig(cfg *Config) {
	b.Config = cfg
}

func (b *BaseGenerator) Generate(ctx context.Context, schema *introspection.Schema) ([]byte, error) {
	return nil, errors.New("not implemented")
}

// FuncMap returns a template.FuncMap that merges both common and generator-specific functions
// The Generator object that's passed will typically consist of a generator data structure
// that also embeds BaseGenerator. BaseGenerator provides a default implementation for most common methods
func FuncMap(g Generator, generatorFunc template.FuncMap) template.FuncMap {
	funcMap := template.FuncMap{
		"FormatName":        g.FormatName,
		"FormatEnum":        g.FormatEnum,
		"FormatDeprecation": g.FormatDeprecation,
		"SortEnumFields":    g.SortEnumFields,
		"IsEnum":            g.IsEnum,
		"Comment":           g.Comment,
		"FormatReturnType":  g.FormatReturnType,
		"FormatInputType":   g.FormatInputType,
		"FormatOutputType":  g.FormatOutputType,
		"ConvertID":         g.ConvertID,
		"IsSelfChainable":   g.IsSelfChainable,
	}

	// Append generator-specific functions:
	for k, v := range generatorFunc {
		funcMap[k] = v
	}
	return funcMap
}

// FormatType loops through the type reference to transform it into its SDK language.
func FormatType(g Generator, r *introspection.TypeRef, input bool) (representation string) {
	for ref := r; ref != nil; ref = ref.OfType {
		switch ref.Kind {
		case introspection.TypeKindList:
			// Handle this special case with defer to format array at the end of
			// the loop.
			// Since an SDK needs to insert it at the end, other at the beginning.
			defer func() {
				representation = g.FormatKindList(representation)
			}()
		case introspection.TypeKindScalar:
			switch introspection.Scalar(ref.Name) {
			case introspection.ScalarString:
				return g.FormatKindScalarString(representation)
			case introspection.ScalarInt:
				return g.FormatKindScalarInt(representation)
			case introspection.ScalarFloat:
				return g.FormatKindScalarFloat(representation)
			case introspection.ScalarBoolean:
				return g.FormatKindScalarBoolean(representation)
			default:
				return g.FormatKindScalarDefault(representation, ref.Name, input)
			}
		case introspection.TypeKindObject:
			return g.FormatKindObject(representation, ref.Name)
		case introspection.TypeKindInputObject:
			return g.FormatKindInputObject(representation, ref.Name)
		case introspection.TypeKindEnum:
			return g.FormatKindEnum(representation, ref.Name)
		}
	}

	panic(r)
}

// FormatEnum formats a GraphQL Enum value into a Go equivalent
// Example: `fooId` -> `FooID`
func (b *BaseGenerator) FormatEnum(s string) string {
	s = strings.ToLower(s)
	return strcase.ToCamel(s)
}

func (b *BaseGenerator) SortEnumFields(s []introspection.EnumValue) []introspection.EnumValue {
	sort.SliceStable(s, func(i, j int) bool {
		return s[i].Name < s[j].Name
	})
	return s
}

// IsEnum checks if the type is actually custom.
func (b *BaseGenerator) IsEnum(t *introspection.Type) bool {
	return t.Kind == introspection.TypeKindEnum &&
		// We ignore the internal GraphQL enums
		!strings.HasPrefix(t.Name, "__")
}

func (b *BaseGenerator) FormatKindEnum(representation string, refName string) string {
	representation += refName
	return representation
}

func (b *BaseGenerator) FormatDeprecation(s string) string {
	return ""
}

func (b *BaseGenerator) Comment(s string) string {
	return ""
}

func (b *BaseGenerator) FormatName(s string) string {
	return ""
}

func (b *BaseGenerator) FormatReturnType(f introspection.Field) string {
	return ""
}

// ConvertID returns true if the field returns an ID that should be
// converted into an object.
func (b *BaseGenerator) ConvertID(f introspection.Field) bool {
	if f.Name == "id" {
		return false
	}
	ref := f.TypeRef
	if ref.Kind == introspection.TypeKindNonNull {
		ref = ref.OfType
	}
	if ref.Kind != introspection.TypeKindScalar {
		return false
	}

	// FIXME: As of now all custom scalars are IDs. If that changes we
	// need to make sure we can tell the difference.
	alias, ok := CustomScalar[ref.Name]

	// FIXME: We don't have a simple way to convert any ID to its
	// corresponding object (in codegen) so for now just return the
	// current instance. Currently, `sync` is the only field where
	// the error is what we care about but more could be added later.
	// To avoid wasting a result, we return the ID which is a leaf value
	// and triggers execution, but then convert to object in the SDK to
	// allow continued chaining. For this, we're assuming the returned
	// ID represents the exact same object but if that changes, we'll
	// need to adjust.
	return ok && alias == f.ParentObject.Name
}

// IsSelfChainable returns true if an object type has any fields that return that same type.
func (b *BaseGenerator) IsSelfChainable(t introspection.Type) bool {
	for _, f := range t.Fields {
		// Only consider fields that return a non-null object.
		if !f.TypeRef.IsObject() || f.TypeRef.Kind != introspection.TypeKindNonNull {
			continue
		}
		if f.TypeRef.OfType.Name == t.Name {
			return true
		}
	}
	return false
}

// SetSchemaParents sets all the parents for the fields.
func SetSchemaParents(schema *introspection.Schema) {
	for _, t := range schema.Types {
		for _, f := range t.Fields {
			f.ParentObject = t
		}
	}
}

// Introspect get the Dagger Schema with the router r.
func Introspect(ctx context.Context, r *router.Router) (*introspection.Schema, error) {
	var response introspection.Response
	_, err := r.Do(ctx, introspection.Query, "", nil, &response)
	if err != nil {
		return nil, fmt.Errorf("error querying the API: %w", err)
	}
	return response.Schema, nil
}

// IntrospectAndGenerate generate the Dagger API with the router r.
func IntrospectAndGenerate(ctx context.Context, r *router.Router, generator Generator) ([]byte, error) {
	schema, err := Introspect(ctx, r)
	if err != nil {
		return nil, err
	}

	return generator.Generate(ctx, schema)
}
