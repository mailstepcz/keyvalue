package keyvalue

import (
	"fmt"
	"reflect"
	"time"
	stduuid "uuid"

	"github.com/google/uuid"
	"github.com/mailstepcz/types"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// PBStructAdapter is a key-value adapter for Protobuf structures.
type PBStructAdapter struct {
	s *structpb.Struct
}

// NewPBStructAdapter creates a new adapter for a map.
func NewPBStructAdapter(obj interface{}) (*PBStructAdapter, error) {
	s, ok := obj.(*structpb.Struct)
	if !ok {
		return nil, fmt.Errorf("type %s isn't a Protobuf structure", reflect.TypeOf(obj))
	}
	return &PBStructAdapter{
		s: s,
	}, nil
}

// EnumFields enumerates all the public fields of the underlying value.
func (a *PBStructAdapter) EnumFields(fn func(string, Value) error) error {
	for k, v := range a.s.GetFields() {
		if err := fn(k, PBValue{value: v}); err != nil {
			return err
		}
	}
	return nil
}

// Set sets the value of a fields.
func (a *PBStructAdapter) Set(name string, value interface{}) error {
	switch v := value.(type) {
	case int:
		a.s.Fields[name] = structpb.NewNumberValue(float64(v))
	case float64:
		a.s.Fields[name] = structpb.NewNumberValue(v)
	case string:
		a.s.Fields[name] = structpb.NewStringValue(v)
	case bool:
		a.s.Fields[name] = structpb.NewBoolValue(v)
	case time.Time:
		a.s.Fields[name] = structpb.NewStringValue(v.Format(time.RFC3339))
	case *timestamppb.Timestamp:
		a.s.Fields[name] = structpb.NewStringValue(v.AsTime().Format(time.RFC3339))
	case uuid.UUID:
		a.s.Fields[name] = structpb.NewStringValue(v.String())
	case stduuid.UUID:
		a.s.Fields[name] = structpb.NewStringValue(v.String())
	case []string:
		converted := make([]interface{}, len(v))
		for i, val := range v {
			converted[i] = val
		}
		list, err := structpb.NewList(converted)
		if err != nil {
			return fmt.Errorf("field '%s' of type '%T': %w", name, value, err)
		}
		a.s.Fields[name] = structpb.NewListValue(list)
	default:
		// if type does not match, check if type is convertible
		rv := reflect.ValueOf(value)
		switch {
		case rv.CanConvert(types.String):
			a.s.Fields[name] = structpb.NewStringValue(rv.Convert(types.String).Interface().(string))
		default:
			return fmt.Errorf("field '%s' of type '%T': %w", name, value, ErrBadType)
		}
	}
	return nil
}

// PBValue wraps a Protobuf value.
type PBValue struct {
	value interface{}
}

// Interface returns the wrapped value.
func (v PBValue) Interface() interface{} {
	switch v := v.value.(type) {
	case *structpb.Struct:
		return v.AsMap()
	case *structpb.ListValue:
		return v.AsSlice()
	case *structpb.Value:
		return v.AsInterface()
	}
	return ErrBadType
}

var (
	_ Adapter = (*PBStructAdapter)(nil)
)
