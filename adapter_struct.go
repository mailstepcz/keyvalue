package keyvalue

import (
	"database/sql"
	"fmt"
	"reflect"
	"time"
	stduuid "uuid"

	"github.com/google/uuid"
	"github.com/mailstepcz/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// StructAdapter is a key-value adapter for structures.
type StructAdapter struct {
	value     reflect.Value
	factory   *FactoryOption
	convertor *ConvertorOption
}

// NewStructAdapter creates a new adapter for a structure..
func NewStructAdapter(obj interface{}, opts ...Option) (*StructAdapter, error) {
	v := reflect.ValueOf(obj)
	if v.Kind() != reflect.Pointer {
		return nil, fmt.Errorf("type %s isn't a pointer to a struct", v.Type())
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("type %s isn't a struct", v.Type())
	}
	var (
		fo *FactoryOption
		co *ConvertorOption
	)
	for _, opt := range opts {
		switch opt := opt.(type) {
		case *FactoryOption:
			fo = opt
		case *ConvertorOption:
			co = opt
		}
	}
	return &StructAdapter{
		value:     v,
		factory:   fo,
		convertor: co,
	}, nil
}

// EnumFields enumerates all the public fields of the underlying value.
func (a *StructAdapter) EnumFields(fn func(string, Value) error) error {
	for i := 0; i < a.value.NumField(); i++ {
		f := a.value.Type().Field(i)
		if f.PkgPath != "" || f.Tag.Get("kv") == "-" {
			continue
		}
		v := a.value.Field(i)
		if err := fn(f.Name, InterfaceValue{value: v.Interface()}); err != nil {
			return err
		}
	}
	return nil
}

// Set sets the value of a field.
func (a *StructAdapter) Set(name string, value interface{}) error {
	sf, ok := a.value.Type().FieldByName(name)
	if !ok {
		return fmt.Errorf("field '%s': %w", name, ErrNoSuchField)
	}
	if sf.Tag.Get("kv") == "-" {
		return nil
	}
	f := a.value.FieldByIndex(sf.Index)
	v := reflect.ValueOf(value)
	var builder func() interface{}
	if a.factory != nil {
		builder = a.factory.Builders[name]
	}
	var customFuncs map[TypePair]func(interface{}) (interface{}, error)
	if a.convertor != nil {
		customFuncs = a.convertor.Funcs
	}
	d, err := destValue(f.Type(), v, builder, customFuncs)
	if err != nil {
		return err
	}
	f.Set(reflect.ValueOf(d))
	return nil
}

func destValue(t reflect.Type, v reflect.Value, builder func() interface{}, customFuncs map[TypePair]func(interface{}) (interface{}, error)) (interface{}, error) {
	switch {
	case v.Type().AssignableTo(t):
		return v.Interface(), nil
	case v.Type().ConvertibleTo(t):
		return v.Convert(t).Interface(), nil
	case t.Kind() == reflect.Interface:
		if builder == nil {
			return nil, ErrNoBuilder
		}
		dst := builder()
		if err := CopyV1(dst, v.Interface()); err != nil {
			return nil, err
		}
		return dst, nil
	case t.Kind() == reflect.Pointer && t.Elem().Kind() == reflect.Struct && v.Type().Kind() == reflect.Pointer && v.Type().Elem().Kind() == reflect.Struct:
		dst := reflect.New(t.Elem())
		if err := CopyV1(dst.Interface(), v.Interface(), &ConvertorOption{Funcs: customFuncs}); err != nil {
			return nil, err
		}
		return dst.Interface(), nil
	case t == types.UUID && v.Type() == types.String:
		s := v.Interface().(string)
		if s == "" {
			return uuid.Nil, nil
		}
		u, err := uuid.Parse(s)
		if err != nil {
			return nil, err
		}
		return u, nil
	case t == types.StdUUID && v.Type() == types.String:
		s := v.Interface().(string)
		if s == "" {
			return stduuid.Nil(), nil
		}
		u, err := stduuid.Parse(s)
		if err != nil {
			return nil, err
		}
		return u, nil
	case t == types.UUID && v.Type() == types.NullUUID:
		var u uuid.UUID
		if v := v.Interface().(uuid.NullUUID); v.Valid {
			u = v.UUID
		}
		return u, nil
	case t == types.NullUUID && v.Type() == types.UUID:
		var u uuid.NullUUID
		if v := v.Interface().(uuid.UUID); v != uuid.Nil {
			u.UUID = v
			u.Valid = true
		}
		return u, nil
	case t == types.UUIDPtr && v.Type() == types.NullUUID:
		var u *uuid.UUID
		if v := v.Interface().(uuid.NullUUID); v.Valid {
			u = &v.UUID
		}
		return u, nil
	case t == types.NullUUID && v.Type() == types.UUIDPtr:
		var u uuid.NullUUID
		if v := v.Interface().(*uuid.UUID); v != nil {
			u.UUID = *v
			u.Valid = true
		}
		return u, nil
	case t == types.Time && v.Type() == types.NullTime:
		var t time.Time
		if v := v.Interface().(sql.NullTime); v.Valid {
			t = v.Time
		}
		return t, nil
	case t == types.NullTime && v.Type() == types.Time:
		var t sql.NullTime
		if v := v.Interface().(time.Time); !v.IsZero() {
			t.Time = v
			t.Valid = true
		}
		return t, nil
	case t == types.TimePtr && v.Type() == types.NullTime:
		var t *time.Time
		if v := v.Interface().(sql.NullTime); v.Valid {
			t = &v.Time
		}
		return t, nil
	case t == types.NullTime && v.Type() == types.TimePtr:
		var t sql.NullTime
		if v := v.Interface().(*time.Time); v != nil {
			t.Time = *v
			t.Valid = true
		}
		return t, nil
	case t == types.String && v.Type() == types.UUID:
		return v.Interface().(uuid.UUID).String(), nil
	case t == types.String && v.Type() == types.StdUUID:
		return v.Interface().(stduuid.UUID).String(), nil
	case t == types.TimestampPtr && v.Type() == types.Time:
		return timestamppb.New(v.Interface().(time.Time)), nil
	case t == types.Time && v.Type() == types.TimestampPtr:
		return v.Interface().(*timestamppb.Timestamp).AsTime(), nil
	case t.Kind() == reflect.String && v.Type() == types.TimestampPtr:
		parsedTime := v.Interface().(*timestamppb.Timestamp).AsTime().Format(time.RFC3339)
		return parsedTime, nil
	case t == types.Time && v.Kind() == reflect.String:
		t, err := time.Parse(time.RFC3339, v.Interface().(string))
		if err != nil {
			return nil, err
		}
		return t, nil
	case t == types.TimestampPtr && v.Kind() == reflect.String:
		t, err := time.Parse(time.RFC3339, v.Interface().(string))
		if err != nil {
			return nil, err
		}
		return timestamppb.New(t), nil
	case t.Kind() == reflect.Slice && v.Kind() == reflect.Slice:
		r := reflect.MakeSlice(t, v.Len(), v.Len())
		for i := 0; i < v.Len(); i++ {
			d, err := destValue(t.Elem(), v.Index(i), nil, customFuncs)
			if err != nil {
				return nil, err
			}
			r.Index(i).Set(reflect.ValueOf(d))
		}
		return r.Interface(), nil
	default:
		if s, ok := v.Interface().(string); ok {
			return s, nil
		}
		return nil, fmt.Errorf("dest '%s', source '%s': %w", t, v.Type(), ErrBadType)
	}
}

var (
	_ Adapter = (*StructAdapter)(nil)
)
