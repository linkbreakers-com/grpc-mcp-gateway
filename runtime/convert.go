package runtime

import (
	"encoding/json"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// DecodeArgs converts MCP tool arguments into a protobuf request message.
func DecodeArgs(args map[string]any, msg proto.Message) error {
	if msg == nil {
		return nil
	}
	if args == nil {
		args = map[string]any{}
	}
	b, err := json.Marshal(args)
	if err != nil {
		return err
	}
	unmarshal := protojson.UnmarshalOptions{
		DiscardUnknown: false,
	}
	return unmarshal.Unmarshal(b, msg)
}

// EncodeProto converts a protobuf response message into a JSON-compatible map.
func EncodeProto(msg proto.Message) (map[string]any, error) {
	if msg == nil {
		return map[string]any{}, nil
	}
	marshal := protojson.MarshalOptions{
		UseProtoNames:   false,
		EmitUnpopulated: false,
	}
	b, err := marshal.Marshal(msg)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}
