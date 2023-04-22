package querybuilder

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// GraphQLMarshaller is an internal interface for marshalling an object into GraphQL.
type GraphQLMarshaller interface {
	// XXX_GraphQLType is an internal function. It returns the native GraphQL type name
	XXX_GraphQLType() string
	// XXX_GraphqlID is an internal function. It returns the underlying type ID
	XXX_GraphQLID(ctx context.Context) (string, error)
}

const (
	GraphQLMarshallerType = "XXX_GraphQLType"
	GraphQLMarshallerID   = "XXX_GraphQLID"
)

var (
	gqlMarshaller reflect.Type

	// Taken from codegen/generator/functions.go
	// Includes also Platform
	customScalar = map[string]struct{}{
		"ContainerID": {},
		"FileID":      {},
		"DirectoryID": {},
		"SecretID":    {},
		"SocketID":    {},
		"CacheID":     {},
		"Platform":    {},
	}
)

func init() {
	gqlMarshaller = reflect.TypeOf((*GraphQLMarshaller)(nil)).Elem()
}

func MarshalGQL(ctx context.Context, v any) (string, error) {
	return marshalValue(ctx, reflect.ValueOf(v))
}

func marshalValue(ctx context.Context, v reflect.Value) (string, error) {
	t := v.Type()

	if t.Implements(gqlMarshaller) {
		return marshalCustom(ctx, v)
	}

	var b strings.Builder
	switch t.Kind() {
	case reflect.Bool:
		b.WriteString(strconv.FormatBool(v.Bool()))
	case reflect.Int:
		b.WriteString(strconv.FormatInt(v.Int(), 10))
	case reflect.String:
		name := t.Name()
		_, found := customScalar[t.Name()]
		if name != "string" && !found {
			b.WriteString(v.String())
		} else {
			b.WriteString(strconv.Quote(v.String()))
		}
	case reflect.Pointer:
		if v.IsNil() {
			b.WriteString("null")
		} else {
			return marshalValue(ctx, v.Elem())
		}
	case reflect.Slice:
		b.WriteRune('[')
		n := v.Len()
		for i := 0; i < n; i++ {
			m, err := marshalValue(ctx, v.Index(i))
			if err != nil {
				return "", err
			}
			if i > 0 {
				b.WriteRune(',')
			}
			b.WriteString(m)
		}
		b.WriteRune(']')
	case reflect.Struct:
		b.WriteRune('{')
		n := v.NumField()
		for i := 0; i < n; i++ {
			f := t.Field(i)
			name := f.Name
			tag := strings.SplitN(f.Tag.Get("json"), ",", 2)[0]
			if tag != "" {
				name = tag
			}
			m, err := marshalValue(ctx, v.Field(i))
			if err != nil {
				return "", err
			}
			if i > 0 {
				b.WriteRune(',')
			}
			b.WriteString(name)
			b.WriteRune(':')
			b.WriteString(m)
		}
		b.WriteRune('}')
	default:
		panic(fmt.Errorf("unsupported argument of kind %s", t.Kind()))
	}
	return b.String(), nil
}

func marshalCustom(ctx context.Context, v reflect.Value) (string, error) {
	result := v.MethodByName(GraphQLMarshallerID).Call([]reflect.Value{
		reflect.ValueOf(ctx),
	})
	if len(result) != 2 {
		panic(result)
	}
	err := result[1].Interface()
	if err != nil {
		return "", err.(error)
	}

	return fmt.Sprintf("%q", result[0].String()), nil
}

func IsZeroValue(value any) bool {
	v := reflect.ValueOf(value)
	kind := v.Type().Kind()
	switch kind {
	case reflect.Pointer:
		return v.IsNil()
	case reflect.Slice, reflect.Array:
		return v.Len() == 0
	default:
		return v.IsZero()
	}
}
