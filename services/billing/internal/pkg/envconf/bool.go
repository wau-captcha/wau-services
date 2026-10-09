package envconf

import (
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Resolver is Resolve as the Kratos config.Resolver of a config message such as conf.Bootstrap.
// A placeholder always fills in a string, and Scan, which decodes with protojson, takes a bool
// field only from a JSON boolean: the strings that land in bool fields are parsed as well. The
// values of the overridable sections may be replaced from the environment; see Override.
func Resolver(message proto.Message, overridable ...string) func(map[string]any) error {
	descriptor := message.ProtoReflect().Descriptor()
	return func(values map[string]any) error {
		if err := Resolve(values); err != nil {
			return err
		}
		Override(values, overridable...)
		return parseBools(values, descriptor, "")
	}
}

// parseBools replaces the strings values holds for the bool fields of message with booleans.
func parseBools(values map[string]any, message protoreflect.MessageDescriptor, path string) error {
	for key, value := range values {
		field := message.Fields().ByName(protoreflect.Name(key))
		if field == nil {
			field = message.Fields().ByJSONName(key)
		}
		if field == nil || field.Cardinality() == protoreflect.Repeated {
			continue
		}
		switch value := value.(type) {
		case string:
			if field.Kind() != protoreflect.BoolKind {
				continue
			}
			parsed, err := strconv.ParseBool(strings.TrimSpace(value))
			if err != nil {
				return fmt.Errorf("config %s%s: %q is not true or false", path, key, value)
			}
			values[key] = parsed
		case map[string]any:
			if field.Kind() != protoreflect.MessageKind {
				continue
			}
			if err := parseBools(value, field.Message(), path+key+"."); err != nil {
				return err
			}
		}
	}
	return nil
}
