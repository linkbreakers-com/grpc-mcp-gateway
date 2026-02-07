package annotations

import (
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/encoding/protowire"
)

const (
	methodOptionFieldNumber  protowire.Number = 51234
	serviceOptionFieldNumber protowire.Number = 51235
)

type ToolOptions struct {
	Name        string
	Title       string
	Description string
	ReadOnly    bool
	Idempotent  bool
	Destructive bool
}

type ServiceOptions struct {
	Name    string
	Version string
}

func ToolFromMethod(method protoreflect.MethodDescriptor) (ToolOptions, bool) {
	opts, ok := method.Options().(*descriptorpb.MethodOptions)
	if !ok || opts == nil {
		return ToolOptions{}, false
	}
	raw := opts.ProtoReflect().GetUnknown()
	ext := findExtension(raw, methodOptionFieldNumber)
	if ext == nil {
		return ToolOptions{}, false
	}
	return parseMethodOptions(ext)
}

func ServiceFromService(service protoreflect.ServiceDescriptor) (ServiceOptions, bool) {
	opts, ok := service.Options().(*descriptorpb.ServiceOptions)
	if !ok || opts == nil {
		return ServiceOptions{}, false
	}
	raw := opts.ProtoReflect().GetUnknown()
	ext := findExtension(raw, serviceOptionFieldNumber)
	if ext == nil {
		return ServiceOptions{}, false
	}
	return parseServiceOptions(ext)
}

func findExtension(unknown []byte, fieldNumber protowire.Number) []byte {
	for len(unknown) > 0 {
		num, typ, n := protowire.ConsumeTag(unknown)
		if n < 0 {
			return nil
		}
		unknown = unknown[n:]

		switch typ {
		case protowire.VarintType:
			_, m := protowire.ConsumeVarint(unknown)
			if m < 0 {
				return nil
			}
			unknown = unknown[m:]
		case protowire.Fixed32Type:
			_, m := protowire.ConsumeFixed32(unknown)
			if m < 0 {
				return nil
			}
			unknown = unknown[m:]
		case protowire.Fixed64Type:
			_, m := protowire.ConsumeFixed64(unknown)
			if m < 0 {
				return nil
			}
			unknown = unknown[m:]
		case protowire.BytesType:
			b, m := protowire.ConsumeBytes(unknown)
			if m < 0 {
				return nil
			}
			if num == fieldNumber {
				return b
			}
			unknown = unknown[m:]
		default:
			return nil
		}
	}
	return nil
}

func parseMethodOptions(raw []byte) (ToolOptions, bool) {
	var toolBytes []byte
	for len(raw) > 0 {
		num, typ, n := protowire.ConsumeTag(raw)
		if n < 0 {
			return ToolOptions{}, false
		}
		raw = raw[n:]
		if num != 1 || typ != protowire.BytesType {
			skip, err := consumeField(typ, raw)
			if err != nil {
				return ToolOptions{}, false
			}
			raw = raw[skip:]
			continue
		}
		b, m := protowire.ConsumeBytes(raw)
		if m < 0 {
			return ToolOptions{}, false
		}
		toolBytes = b
		break
	}
	if toolBytes == nil {
		return ToolOptions{}, false
	}
	tool := parseToolOptions(toolBytes)
	return tool, true
}

func parseServiceOptions(raw []byte) (ServiceOptions, bool) {
	var out ServiceOptions
	for len(raw) > 0 {
		num, typ, n := protowire.ConsumeTag(raw)
		if n < 0 {
			return ServiceOptions{}, false
		}
		raw = raw[n:]
		switch num {
		case 1:
			if typ != protowire.BytesType {
				return ServiceOptions{}, false
			}
			b, m := protowire.ConsumeBytes(raw)
			if m < 0 {
				return ServiceOptions{}, false
			}
			out.Name = string(b)
			raw = raw[m:]
		case 2:
			if typ != protowire.BytesType {
				return ServiceOptions{}, false
			}
			b, m := protowire.ConsumeBytes(raw)
			if m < 0 {
				return ServiceOptions{}, false
			}
			out.Version = string(b)
			raw = raw[m:]
		default:
			skip, err := consumeField(typ, raw)
			if err != nil {
				return ServiceOptions{}, false
			}
			raw = raw[skip:]
		}
	}
	return out, true
}

func parseToolOptions(raw []byte) ToolOptions {
	var out ToolOptions
	for len(raw) > 0 {
		num, typ, n := protowire.ConsumeTag(raw)
		if n < 0 {
			return out
		}
		raw = raw[n:]
		switch num {
		case 1:
			if typ != protowire.BytesType {
				return out
			}
			b, m := protowire.ConsumeBytes(raw)
			if m < 0 {
				return out
			}
			out.Name = string(b)
			raw = raw[m:]
		case 2:
			if typ != protowire.BytesType {
				return out
			}
			b, m := protowire.ConsumeBytes(raw)
			if m < 0 {
				return out
			}
			out.Title = string(b)
			raw = raw[m:]
		case 3:
			if typ != protowire.BytesType {
				return out
			}
			b, m := protowire.ConsumeBytes(raw)
			if m < 0 {
				return out
			}
			out.Description = string(b)
			raw = raw[m:]
		case 4:
			if typ != protowire.VarintType {
				return out
			}
			v, m := protowire.ConsumeVarint(raw)
			if m < 0 {
				return out
			}
			out.ReadOnly = v != 0
			raw = raw[m:]
		case 5:
			if typ != protowire.VarintType {
				return out
			}
			v, m := protowire.ConsumeVarint(raw)
			if m < 0 {
				return out
			}
			out.Idempotent = v != 0
			raw = raw[m:]
		case 6:
			if typ != protowire.VarintType {
				return out
			}
			v, m := protowire.ConsumeVarint(raw)
			if m < 0 {
				return out
			}
			out.Destructive = v != 0
			raw = raw[m:]
		default:
			skip, err := consumeField(typ, raw)
			if err != nil {
				return out
			}
			raw = raw[skip:]
		}
	}
	return out
}

func consumeField(typ protowire.Type, raw []byte) (int, error) {
	switch typ {
	case protowire.VarintType:
		_, m := protowire.ConsumeVarint(raw)
		return m, protowire.ParseError(m)
	case protowire.Fixed32Type:
		_, m := protowire.ConsumeFixed32(raw)
		return m, protowire.ParseError(m)
	case protowire.Fixed64Type:
		_, m := protowire.ConsumeFixed64(raw)
		return m, protowire.ParseError(m)
	case protowire.BytesType:
		_, m := protowire.ConsumeBytes(raw)
		return m, protowire.ParseError(m)
	default:
		return 0, protowire.ParseError(-1)
	}
}
