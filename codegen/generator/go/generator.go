package gogenerator

import (
	"bytes"
	"context"
	"fmt"
	"go/format"
	"strings"
	"text/template"
	"unicode"

	"github.com/dagger/dagger/codegen/generator"
	"github.com/dagger/dagger/codegen/introspection"

	_ "embed"
)

var (
	//go:embed templates/src/header.go.tmpl
	headerSource string

	//go:embed templates/src/scalar.go.tmpl
	scalarSource string

	//go:embed templates/src/input.go.tmpl
	inputSource string

	//go:embed templates/src/object.go.tmpl
	objectSource string

	//go:embed templates/src/enum.go.tmpl
	enumSource string
)

type GoGenerator struct {
	generator.BaseGenerator

	headerTpl *template.Template
	scalarTpl *template.Template
	inputTpl  *template.Template
	objectTpl *template.Template
	enumTpl   *template.Template
}

func (g *GoGenerator) LoadTemplates() (*template.Template, error) {
	funcMap := generator.FuncMap(g, g.CommonFunc, template.FuncMap{
		"FieldOptionsStructName":  g.fieldOptionsStructName,
		"FieldFunction":           g.fieldFunction,
		"IsListOfObject":          g.isListOfObject,
		"GetArrayField":           g.getArrayField,
		"ToLowerCase":             g.toLowerCase,
		"ToUpperCase":             g.toUpperCase,
		"FormatArrayField":        g.formatArrayField,
		"FormatArrayToSingleType": g.formatArrayToSingleType,
		"FormatDeprecation":       g.FormatDeprecation,
		"comment":                 g.Comment,
	})

	var err error
	g.headerTpl, err = template.New("header").Funcs(funcMap).Parse(headerSource)
	if err != nil {
		return nil, err
	}
	g.scalarTpl, err = template.New("scalar").Funcs(funcMap).Parse(scalarSource)
	if err != nil {
		return nil, err
	}

	g.inputTpl, err = template.New("input").Funcs(funcMap).Parse(inputSource)
	if err != nil {
		return nil, err
	}

	g.objectTpl, err = template.New("object").Funcs(funcMap).Parse(objectSource)
	if err != nil {
		return nil, err
	}

	g.enumTpl, err = template.New("enum").Funcs(funcMap).Parse(enumSource)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// FormatName formats a GraphQL name (e.g. object, field, arg) into a Go equivalent
// Example: `fooId` -> `FooID`
func (g *GoGenerator) FormatName(s string) string {
	if len(s) > 0 {
		s = strings.ToUpper(string(s[0])) + s[1:]
	}
	return g.lintName(s)
}

// lintName returns a different name if it should be different.
func (g *GoGenerator) lintName(name string) (should string) {
	// Fast path for simple cases: "_" and all lowercase.
	if name == "_" {
		return name
	}
	allLower := true
	for _, r := range name {
		if !unicode.IsLower(r) {
			allLower = false
			break
		}
	}
	if allLower {
		return name
	}

	// Split camelCase at any lower->upper transition, and split on underscores.
	// Check each word for common initialisms.
	runes := []rune(name)
	w, i := 0, 0 // index of start of word, scan
	for i+1 <= len(runes) {
		eow := false // whether we hit the end of a word
		if i+1 == len(runes) {
			eow = true
		} else if runes[i+1] == '_' {
			// underscore; shift the remainder forward over any run of underscores
			eow = true
			n := 1
			for i+n+1 < len(runes) && runes[i+n+1] == '_' {
				n++
			}

			// Leave at most one underscore if the underscore is between two digits
			if i+n+1 < len(runes) && unicode.IsDigit(runes[i]) && unicode.IsDigit(runes[i+n+1]) {
				n--
			}

			copy(runes[i+1:], runes[i+n+1:])
			runes = runes[:len(runes)-n]
		} else if unicode.IsLower(runes[i]) && !unicode.IsLower(runes[i+1]) {
			// lower->non-lower
			eow = true
		}
		i++
		if !eow {
			continue
		}

		// [w,i) is a word.
		word := string(runes[w:i])
		if u := strings.ToUpper(word); commonInitialisms[u] {
			// Keep consistent case, which is lowercase only at the start.
			if w == 0 && unicode.IsLower(runes[w]) {
				u = strings.ToLower(u)
			}
			// All the common initialisms are ASCII,
			// so we can replace the bytes exactly.
			copy(runes[w:], []rune(u))
		} else if w > 0 && strings.ToLower(word) == word {
			// already all lowercase, and not the first word, so uppercase the first character.
			runes[w] = unicode.ToUpper(runes[w])
		}
		w = i
	}
	return string(runes)
}

// commonInitialisms is a set of common initialisms.
// Only add entries that are highly unlikely to be non-initialisms.
// For instance, "ID" is fine (Freudian code is rare), but "AND" is not.
var commonInitialisms = map[string]bool{
	"ACL":   true,
	"API":   true,
	"ASCII": true,
	"CPU":   true,
	"CSS":   true,
	"DNS":   true,
	"EOF":   true,
	"GUID":  true,
	"HTML":  true,
	"HTTP":  true,
	"HTTPS": true,
	"ID":    true,
	"IP":    true,
	"JSON":  true,
	"LHS":   true,
	"QPS":   true,
	"RAM":   true,
	"RHS":   true,
	"RPC":   true,
	"SLA":   true,
	"SMTP":  true,
	"SQL":   true,
	"SSH":   true,
	"TCP":   true,
	"TLS":   true,
	"TTL":   true,
	"UDP":   true,
	"UI":    true,
	"UID":   true,
	"UUID":  true,
	"URI":   true,
	"URL":   true,
	"UTF8":  true,
	"VM":    true,
	"XML":   true,
	"XMPP":  true,
	"XSRF":  true,
	"XSS":   true,

	// Custom added for dagger
	"FS":  true,
	"SDK": true,
}

// format the deprecation reason
// Example: `Replaced by @foo.` -> `// Replaced by Foo\n`
func (g *GoGenerator) FormatDeprecation(s string) string {
	matches := generator.FormatDeprecationRegex.FindAllString(s, -1)
	for _, match := range matches {
		replacement := strings.TrimPrefix(match, "`")
		replacement = strings.TrimSuffix(replacement, "`")
		replacement = g.FormatName(replacement)
		s = strings.ReplaceAll(s, match, replacement)
	}
	return g.Comment("Deprecated: " + s)
}

func (g *GoGenerator) FormatKindList(representation string) string {
	representation = "[]" + representation
	return representation
}

// Comment comments out a string
// Example: `hello\nworld` -> `// hello\n// world\n`
func (g *GoGenerator) Comment(s string) string {
	if s == "" {
		return ""
	}

	lines := strings.Split(s, "\n")

	for i, l := range lines {
		lines[i] = "// " + l
	}
	return strings.Join(lines, "\n")
}

// fieldOptionsStructName returns the options struct name for a given field
func (g *GoGenerator) fieldOptionsStructName(f introspection.Field) string {
	// Exception: `Query` option structs are not prefixed by `Query`.
	// This is just so that they're nicer to work with, e.g.
	// `ContainerOpts` rather than `QueryContainerOpts`
	// The structure name will not clash with others since everybody else
	// is prefixed by object name.
	if f.ParentObject.Name == generator.QueryStructName {
		return g.FormatName(f.Name) + "Opts"
	}
	return g.FormatName(f.ParentObject.Name) + g.FormatName(f.Name) + "Opts"
}

// fieldFunction converts a field into a function signature
// Example: `contents: String!` -> `func (r *File) Contents(ctx context.Context) (string, error)`
func (g *GoGenerator) fieldFunction(f introspection.Field) string {
	structName := g.FormatName(f.ParentObject.Name)
	if structName == generator.QueryStructName {
		structName = "Client"
	}
	signature := fmt.Sprintf(`func (r *%s) %s`,
		structName, g.FormatName(f.Name))

	// Generate arguments
	args := []string{}
	if f.TypeRef.IsScalar() || f.TypeRef.IsList() {
		args = append(args, "ctx context.Context")
	}
	for _, arg := range f.Args {
		if arg.TypeRef.IsOptional() {
			continue
		}

		// FIXME: For top-level queries (e.g. File, Directory) if the field is named `id` then keep it as a
		// scalar (DirectoryID) rather than an object (*Directory).
		if f.ParentObject.Name == generator.QueryStructName && arg.Name == "id" {
			args = append(args, fmt.Sprintf("%s %s", arg.Name, g.CommonFunc.FormatOutputType(arg.TypeRef)))
		} else {
			args = append(args, fmt.Sprintf("%s %s", arg.Name, g.CommonFunc.FormatInputType(arg.TypeRef)))
		}
	}
	// Options (e.g. DirectoryContentsOptions -> <Object><Field>Options)
	if f.Args.HasOptionals() {
		args = append(
			args,
			fmt.Sprintf("opts ...%s", g.fieldOptionsStructName(f)),
		)
	}
	signature += "(" + strings.Join(args, ", ") + ")"

	retType := g.CommonFunc.FormatReturnType(f)
	if f.TypeRef.IsScalar() || f.TypeRef.IsList() {
		retType = fmt.Sprintf("(%s, error)", retType)
	} else {
		retType = "*" + retType
	}
	signature += " " + retType

	return signature
}

func (g *GoGenerator) isListOfObject(t *introspection.TypeRef) bool {
	return t.OfType.OfType.IsObject()
}

func (g *GoGenerator) getArrayField(f *introspection.Field) []*introspection.Field {
	schema := generator.GetSchema()

	fieldType := f.TypeRef
	if !fieldType.IsOptional() {
		fieldType = fieldType.OfType
	}
	if !fieldType.IsList() {
		panic("field is not a list")
	}
	fieldType = fieldType.OfType
	if !fieldType.IsOptional() {
		fieldType = fieldType.OfType
	}
	schemaType := schema.Types.Get(fieldType.Name)
	if schemaType == nil {
		panic(fmt.Sprintf("schema type %s is nil", fieldType.Name))
	}

	var fields []*introspection.Field
	// Only include scalar fields for now
	// TODO: include subtype too
	for _, typeField := range schemaType.Fields {
		if typeField.TypeRef.IsScalar() {
			fields = append(fields, typeField)
		}
	}

	return fields
}

func (g *GoGenerator) toLowerCase(s string) string {
	return fmt.Sprintf("%c%s", unicode.ToLower(rune(s[0])), s[1:])
}

func (g *GoGenerator) toUpperCase(s string) string {
	return fmt.Sprintf("%c%s", unicode.ToUpper(rune(s[0])), s[1:])
}

func (g *GoGenerator) formatArrayToSingleType(arrType string) string {
	return arrType[2:]
}

func (g *GoGenerator) formatArrayField(fields []*introspection.Field) string {
	result := []string{}

	for _, f := range fields {
		result = append(result, fmt.Sprintf("%s: &fields[i].%s", f.Name, g.toUpperCase(f.Name)))
	}

	return strings.Join(result, ", ")
}

func (g *GoGenerator) Generate(_ context.Context, schema *introspection.Schema) ([]byte, error) {
	generator.SetSchema(schema)

	// g.CommonFunc = generator.NewCommonFunctions(&templates.FormatTypeFunc{})

	if _, err := g.LoadTemplates(); err != nil {
		return nil, err
	}

	headerData := struct {
		Package string
		Schema  *introspection.Schema
	}{
		Package: g.Config.Package,
		Schema:  schema,
	}
	var header bytes.Buffer
	if err := g.headerTpl.Execute(&header, headerData); err != nil {
		return nil, err
	}

	render := []string{
		header.String(),
	}

	err := schema.Visit(introspection.VisitHandlers{
		Scalar: func(t *introspection.Type) error {
			var out bytes.Buffer
			if err := g.scalarTpl.Execute(&out, t); err != nil {
				return err
			}
			render = append(render, out.String())
			return nil
		},
		Object: func(t *introspection.Type) error {
			var out bytes.Buffer
			if err := g.objectTpl.Execute(&out, t); err != nil {
				return err
			}
			render = append(render, out.String())
			return nil
		},
		Enum: func(t *introspection.Type) error {
			var out bytes.Buffer
			if err := g.enumTpl.Execute(&out, t); err != nil {
				return err
			}
			render = append(render, out.String())
			return nil
		},
		Input: func(t *introspection.Type) error {
			var out bytes.Buffer
			if err := g.inputTpl.Execute(&out, t); err != nil {
				return err
			}
			render = append(render, out.String())
			return nil
		},
	})
	if err != nil {
		return nil, err
	}

	formatted, err := format.Source(
		[]byte(strings.Join(render, "\n")),
	)
	if err != nil {
		return nil, fmt.Errorf("error formatting generated code: %w", err)
	}
	return formatted, nil
}
