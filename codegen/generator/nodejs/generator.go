package nodegenerator

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/dagger/dagger/codegen/generator"
	"github.com/dagger/dagger/codegen/generator/nodejs/templates"
	"github.com/dagger/dagger/codegen/introspection"
	"github.com/iancoleman/strcase"

	_ "embed"
)

//go:embed templates/src
var srcs embed.FS

type NodeGenerator struct {
	generator.BaseGenerator
}

// formatName formats a GraphQL name (e.g. object, field, arg) into a TS equivalent
func (g *NodeGenerator) formatName(s string) string {
	return s
}

func (g *NodeGenerator) FormatKindObject(representation string, refName string) string {
	name := refName
	if name == generator.QueryStructName {
		name = generator.QueryStructClientName
	}

	representation += g.formatName(name)
	return representation
}

func (g *NodeGenerator) FormatKindInputObject(representation string, refName string) string {
	representation += g.formatName(refName)
	return representation
}

func (g *NodeGenerator) FormatKindEnum(representation string, refName string) string {
	representation += refName
	return representation
}

// commentToLines split a string by line breaks to be used in comments
func (g *NodeGenerator) commentToLines(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{}
	}

	split := strings.Split(s, "\n")
	return split
}

// solve checks if a field is solveable.
func (g *NodeGenerator) solve(field introspection.Field) bool {
	if field.TypeRef == nil {
		return false
	}
	return field.TypeRef.IsScalar() || field.TypeRef.IsList()
}

func (g *NodeGenerator) splitRequiredOptionalArgs(values introspection.InputValues) (required introspection.InputValues, optionals introspection.InputValues) {
	for i, v := range values {
		if v.TypeRef != nil && !v.TypeRef.IsOptional() {
			continue
		}
		return values[:i], values[i:]
	}
	return values, nil
}

func (g *NodeGenerator) getRequiredArgs(values introspection.InputValues) introspection.InputValues {
	required, _ := g.splitRequiredOptionalArgs(values)
	return required
}

func (g *NodeGenerator) getOptionalArgs(values introspection.InputValues) introspection.InputValues {
	_, optional := g.splitRequiredOptionalArgs(values)
	return optional
}

// subtract subtract integer a with integer b.
func (g *NodeGenerator) subtract(a, b int) int {
	return a - b
}

func (g *NodeGenerator) argsHaveDescription(values introspection.InputValues) bool {
	for _, o := range values {
		if strings.TrimSpace(o.Description) != "" {
			return true
		}
	}

	return false
}

// isCustomScalar checks if the type is actually custom.
func (g *NodeGenerator) isCustomScalar(t *introspection.Type) bool {
	switch introspection.Scalar(t.Name) {
	case introspection.ScalarString, introspection.ScalarInt, introspection.ScalarFloat, introspection.ScalarBoolean:
		return false
	default:
		return t.Kind == introspection.TypeKindScalar
	}
}

func (g *NodeGenerator) sortInputFields(s []introspection.InputValue) []introspection.InputValue {
	sort.SliceStable(s, func(i, j int) bool {
		return s[i].Name < s[j].Name
	})
	return s
}

func (g *NodeGenerator) sortEnumFields(s []introspection.EnumValue) []introspection.EnumValue {
	sort.SliceStable(s, func(i, j int) bool {
		return s[i].Name < s[j].Name
	})
	return s
}

// format the deprecation reason
// Example: `Replaced by @foo.` -> `// Replaced by Foo\n`
func (g *NodeGenerator) formatDeprecation(s string) []string {
	matches := generator.FormatDeprecationRegex.FindAllString(s, -1)
	for _, match := range matches {
		replacement := strings.TrimPrefix(match, "`")
		replacement = strings.TrimSuffix(replacement, "`")
		replacement = g.formatName(replacement)
		s = strings.ReplaceAll(s, match, replacement)
	}
	return g.commentToLines("@deprecated " + s)
}

func (g *NodeGenerator) LoadTemplates() (*template.Template, error) {
	topLevelTemplate := "api"
	templateDeps := []string{
		topLevelTemplate, "header", "objects", "object", "method", "method_solve", "call_args", "method_comment", "types", "args",
	}

	fileNames := make([]string, 0, len(templateDeps))
	for _, tmpl := range templateDeps {
		fileNames = append(fileNames, fmt.Sprintf("templates/src/%s.ts.gtpl", tmpl))
	}

	funcMap := generator.FuncMap(g, g.CommonFunc, template.FuncMap{
		"FormatDeprecation":   g.formatDeprecation,
		"HasPrefix":           strings.HasPrefix,
		"CommentToLines":      g.commentToLines,
		"Solve":               g.solve,
		"GetRequiredArgs":     g.getRequiredArgs,
		"GetOptionalArgs":     g.getOptionalArgs,
		"PascalCase":          strcase.ToCamel,
		"Subtract":            g.subtract,
		"ArgsHaveDescription": g.argsHaveDescription,
		"IsCustomScalar":      g.isCustomScalar,
		"SortInputFields":     g.sortInputFields,
		"SortEnumFields":      g.sortEnumFields,
	})

	// TODO: fix
	// funcMap := template.FuncMap{}
	return template.New(topLevelTemplate).Funcs(funcMap).ParseFS(srcs, fileNames...)
}

// Generate will generate the NodeJS SDK code and might modify the schema to reorder types in a alphanumeric fashion.
func (g *NodeGenerator) Generate(_ context.Context, schema *introspection.Schema) ([]byte, error) {
	g.CommonFunc = generator.NewCommonFunctions(&templates.FormatTypeFunc{})

	sort.SliceStable(schema.Types, func(i, j int) bool {
		return schema.Types[i].Name < schema.Types[j].Name
	})
	for _, v := range schema.Types {
		sort.SliceStable(v.Fields, func(i, j int) bool {
			return v.Fields[i].Name < v.Fields[j].Name
		})
	}

	tmpl, err := g.LoadTemplates()
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	if err := tmpl.ExecuteTemplate(&b, "api", schema.Types); err != nil {
		return nil, err
	}

	return b.Bytes(), err
}
