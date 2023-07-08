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
)

type Config struct {
	Lang SDKLang
	// Package is the target package that is generated.
	// Not used for the SDKLangNodeJS.
	Package string
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
	FormatReturnType(introspection.Field) string
	FormatKindList(string) string
}

// BaseGenerator provides default implementations for common methods
// It also holds the generator configuration data structure
// CommonFunc is kept for compatibility with previous codebase:
type BaseGenerator struct {
	Config     *Config
	CommonFunc *CommonFunctions
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
func FuncMap(g Generator, commonFunc *CommonFunctions, generatorFunc template.FuncMap) template.FuncMap {
	funcMap := template.FuncMap{
		"FormatName":        g.FormatName,
		"FormatEnum":        g.FormatEnum,
		"FormatDeprecation": g.FormatDeprecation,
		"SortEnumFields":    g.SortEnumFields,
		"IsEnum":            g.IsEnum,
		"Comment":           g.Comment,
		"FormatReturnType":  commonFunc.FormatReturnType,
		"FormatInputType":   commonFunc.FormatInputType,
		"FormatOutputType":  commonFunc.FormatOutputType,
		"ConvertID":         commonFunc.ConvertID,
	}

	// Append generator-specific functions:
	for k, v := range generatorFunc {
		funcMap[k] = v
	}
	return funcMap
}

// formatType loops through the type reference to transform it into its SDK language.
func formatType(g Generator, r *introspection.TypeRef, input bool) (representation string) {
	for ref := r; ref != nil; ref = ref.OfType {
		switch ref.Kind {
		case introspection.TypeKindList:
			// Handle this special case with defer to format array at the end of
			// the loop.
			// Since an SDK needs to insert it at the end, other at the beginning.
			defer func() {
				// representation = c.formatTypeFuncs.FormatKindList(representation)
				representation = g.FormatKindList(representation)
			}()
		case introspection.TypeKindScalar:
			switch introspection.Scalar(ref.Name) {
			case introspection.ScalarString:
				// return c.formatTypeFuncs.FormatKindScalarString(representation)
			case introspection.ScalarInt:
				// return c.formatTypeFuncs.FormatKindScalarInt(representation)
			case introspection.ScalarFloat:
				// return c.formatTypeFuncs.FormatKindScalarFloat(representation)
			case introspection.ScalarBoolean:
				// return c.formatTypeFuncs.FormatKindScalarBoolean(representation)
			default:
				// return c.formatTypeFuncs.FormatKindScalarDefault(representation, ref.Name, input)
			}
		case introspection.TypeKindObject:
			// return c.formatTypeFuncs.FormatKindObject(representation, ref.Name)
		case introspection.TypeKindInputObject:
			// return c.formatTypeFuncs.FormatKindInputObject(representation, ref.Name)
		case introspection.TypeKindEnum:
			// return c.formatTypeFuncs.FormatKindEnum(representation, ref.Name)
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
