// Package codec replaces the Kratos "json" codec so proto messages are encoded with
// protojson: proto field names, RFC 3339 timestamps and explicit zero values.
// Import it for side effects.
package codec

import (
	"github.com/go-kratos/kratos/v3/encoding"
	kjson "github.com/go-kratos/kratos/v3/encoding/json"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var (
	marshalOptions   = protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}
	unmarshalOptions = protojson.UnmarshalOptions{DiscardUnknown: true}
)

func init() {
	encoding.RegisterCodec(jsonCodec{encoding.GetCodec(kjson.Name)})
}

// jsonCodec uses protojson for proto messages and the Kratos codec for anything else.
type jsonCodec struct {
	encoding.Codec
}

func (c jsonCodec) Marshal(v any) ([]byte, error) {
	if m, ok := v.(proto.Message); ok {
		return marshalOptions.Marshal(m)
	}
	return c.Codec.Marshal(v)
}

func (c jsonCodec) Unmarshal(data []byte, v any) error {
	if m, ok := v.(proto.Message); ok && len(data) > 0 {
		return unmarshalOptions.Unmarshal(data, m)
	}
	return c.Codec.Unmarshal(data, v)
}
