// Package keyvalue provides the [Copy] function that copies data between structures.
// The functionality includes implicit and custom conversions.
package keyvalue

import (
	"errors"
	"reflect"
	"slices"
	"sync"
	"time"
	"unsafe"

	"github.com/google/uuid"
	"github.com/mailstepcz/enums"
	"github.com/mailstepcz/maybe"
	"github.com/mailstepcz/must"
	"github.com/mailstepcz/pointer"
	"github.com/mailstepcz/serr"
	"github.com/mailstepcz/slice"
	"github.com/mailstepcz/types"
	"github.com/mailstepcz/types/iface"
	"github.com/mailstepcz/validate"
	"github.com/oklog/ulid/v2"
	"github.com/rickb777/date/v2"
	"github.com/shopspring/decimal"
	"golang.org/x/text/language"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	// ErrTypeNotStruct signifies that the type is not a structure.
	ErrTypeNotStruct = errors.New("type not struct")
	// ErrFieldNotFound signifies that the required field was not found.
	ErrFieldNotFound = errors.New("field not found")
	// ErrUnsupportedTypePair signifies incompatible type pair.
	ErrUnsupportedTypePair = errors.New("unsupported pair")
	// ErrPointerNotSupportedInDestinationSlice signifies that a pointer in the slice would clash with the GC.
	ErrPointerNotSupportedInDestinationSlice = errors.New("dangerous pointer in slice")

	copiers  = make(map[copierTypePair]func(unsafe.Pointer, unsafe.Pointer) error)
	cacheMtx sync.RWMutex
)

var (
	dynmapType = reflect.TypeFor[map[string]interface{}]()
)

// CopierOptions defines copier options.
type CopierOptions struct {
	OmitNotFound bool
	FieldsToCopy []string
	FieldsToOmit []string
}

type copierTypePair struct {
	dst, src reflect.Type
}

// CopierForPair creates a copier for a pair of structs.
func CopierForPair(dstType, srcType reflect.Type) (func(unsafe.Pointer, unsafe.Pointer) error, error) {
	return CopierForPairWithOptions(dstType, srcType, nil)
}

// CopierForPairWithOptions creates a copier for a pair of structs with custom options.
func CopierForPairWithOptions(dstType, srcType reflect.Type, opts *CopierOptions) (func(unsafe.Pointer, unsafe.Pointer) error, error) {
	key := copierTypePair{
		dst: dstType,
		src: srcType,
	}
	cacheMtx.RLock()
	copier, ok := copiers[key]
	cacheMtx.RUnlock()
	if ok {
		return copier, nil
	}
	if dstType.Kind() != reflect.Struct || srcType.Kind() != reflect.Struct {
		return nil, ErrTypeNotStruct
	}
	fieldCopiers := make([]func(unsafe.Pointer, unsafe.Pointer) error, 0, srcType.NumField())
	for _, srcField := range reflect.VisibleFields(srcType) {
		if srcField.PkgPath != "" {
			continue
		}
		if srcField.Tag.Get("kv") == "-" {
			continue
		}
		if opts != nil {
			if slices.Index(opts.FieldsToOmit, srcField.Name) != -1 {
				continue
			}
			if opts.FieldsToCopy != nil && slices.Index(opts.FieldsToCopy, srcField.Name) == -1 {
				continue
			}
		}
		dstField, ok := dstType.FieldByName(srcField.Name)
		if !ok {
			if opts != nil && opts.OmitNotFound {
				continue
			}
			return nil, serr.Wrap("", ErrFieldNotFound, serr.String("srcField", srcField.Name), serr.String("srcType", srcType.Name()))
		}
		fc, err := fieldCopier(dstField.Type, srcField.Type, dstField.Offset, srcField.Offset)
		if err != nil {
			return nil, serr.Wrap("", err, serr.String("srcField", srcField.Name))
		}
		fieldCopiers = append(fieldCopiers, fc)
	}
	fieldCopiers = slices.Clip(fieldCopiers)
	copier = func(dst, src unsafe.Pointer) error {
		for _, fc := range fieldCopiers {
			if err := fc(dst, src); err != nil {
				return err
			}
		}
		return nil
	}
	cacheMtx.Lock()
	defer cacheMtx.Unlock()
	copiers[key] = copier
	return copier, nil
}

func memcopy(dst, src unsafe.Pointer, size uintptr) {
	switch size {
	case 8:
		*(*[8]byte)(dst) = *(*[8]byte)(src)
	case 16:
		*(*[16]byte)(dst) = *(*[16]byte)(src)
	case 24:
		*(*[24]byte)(dst) = *(*[24]byte)(src)
	default:
		copy(unsafe.Slice((*byte)(dst), size), unsafe.Slice((*byte)(src), size))
	}
}

// ValueCopier returns a copier for values of any type (provided the pair of types is supported).
func ValueCopier[D, S any]() (func(*D, *S) error, error) {
	c, err := valConv(reflect.TypeFor[D](), reflect.TypeFor[S]())
	if err != nil {
		return nil, err
	}
	return func(dst *D, src *S) error {
		return c(unsafe.Pointer(dst), unsafe.Pointer(src))
	}, nil
}

// Copier provides a method for copying values of different types.
type Copier[D, S any] interface {
	Copy(S) (D, error)
	CopyPtr(*S) (*D, error)
}

// Copying wraps copier functions to conform to the [Copier] interface.
type Copying[D, S any] func(*D, *S) error

func (f Copying[D, S]) Copy(src S) (D, error) {
	var dst D
	if err := f(&dst, &src); err != nil {
		return dst, err
	}
	return dst, nil
}

func (f Copying[D, S]) CopyPtr(src *S) (*D, error) {
	var dst D
	if err := f(&dst, src); err != nil {
		return nil, err
	}
	return &dst, nil
}

// Cast is a convenience function for obtaining an instance of [Copying].
func Cast[D, S any](f func(*D, *S) error) Copying[D, S] {
	return Copying[D, S](f)
}

func valConv(dstType, srcType reflect.Type) (func(unsafe.Pointer, unsafe.Pointer) error, error) {
	dstPtrType := reflect.PointerTo(dstType)
	srcPtrType := reflect.PointerTo(srcType)
	switch {
	case dstType == srcType:
		size := dstType.Size()
		return func(dst, src unsafe.Pointer) error {
			memcopy(dst, src, size)
			return nil
		}, nil

	case dstType.Kind() == reflect.String && srcType.Kind() == reflect.String && dstType.Implements(types.ClosedEnum):
		validator := func(x string) error {
			e := reflect.ValueOf(x).Convert(dstType).Interface().(enums.ClosedEnum)
			if e.EnumValueIsValid() {
				return nil
			}
			return serr.New("bad value for closed enum", serr.String("value", x), serr.String("dstType", dstType.Name()))
		}
		return func(dst, src unsafe.Pointer) error {
			x := (*string)(src)
			if err := validator(*x); err != nil {
				return err
			}
			y := (*string)(dst)
			*y = *x
			return nil
		}, nil

	case srcPtrType.ConvertibleTo(dstPtrType):
		return func(dst, src unsafe.Pointer) error {
			converted := reflect.NewAt(srcType, src).Convert(dstPtrType)
			copy(unsafe.Slice((*byte)(dst), dstType.Size()), unsafe.Slice((*byte)(converted.UnsafePointer()), dstType.Size()))
			return nil
		}, nil

	case srcType.ConvertibleTo(dstType):
		return func(dst, src unsafe.Pointer) error {
			converted := reflect.NewAt(srcType, src).Elem().Convert(dstType)
			if converted.CanAddr() {
				copy(unsafe.Slice((*byte)(dst), dstType.Size()), unsafe.Slice((*byte)(converted.Addr().UnsafePointer()), dstType.Size()))
			} else {
				reflect.NewAt(dstType, dst).Elem().Set(converted)
			}
			return nil
		}, nil

	case srcType == types.Time && dstType == types.TimestampPtr:
		return func(dst, src unsafe.Pointer) error {
			t := *(*time.Time)(src)
			*(**timestamppb.Timestamp)(dst) = timestamppb.New(t)
			return nil
		}, nil

	case srcType == types.TimePtr && dstType == types.TimestampPtr:
		return func(dst, src unsafe.Pointer) error {
			if t := *(**time.Time)(src); t != nil {
				*(**timestamppb.Timestamp)(dst) = timestamppb.New(*t)
			}
			return nil
		}, nil

	case dstType == types.Time && srcType == types.TimestampPtr:
		return func(dst, src unsafe.Pointer) error {
			if ts := *(**timestamppb.Timestamp)(src); ts.IsValid() {
				*(*time.Time)(dst) = ts.AsTime()
			}
			return nil
		}, nil

	case dstType == types.TimePtr && srcType == types.TimestampPtr:
		return func(dst, src unsafe.Pointer) error {
			if ts := *(**timestamppb.Timestamp)(src); ts.IsValid() {
				*(**time.Time)(dst) = pointer.To(ts.AsTime())
			}
			return nil
		}, nil

	case srcType == types.UUID && dstType == types.String:
		return func(dst, src unsafe.Pointer) error {
			x := (*uuid.UUID)(src)
			*(*string)(dst) = x.String()
			return nil
		}, nil

	case srcType == types.TimestampPtr && dstType == types.Date:
		return func(dst, src unsafe.Pointer) error {
			if ts := *(**timestamppb.Timestamp)(src); ts.IsValid() {
				*(*date.Date)(dst) = date.NewAt(ts.AsTime())
			}
			return nil
		}, nil

	case srcType == types.Date && dstType == types.TimestampPtr:
		return func(dst, src unsafe.Pointer) error {
			x := *(*date.Date)(src)
			*(**timestamppb.Timestamp)(dst) = timestamppb.New(x.MidnightUTC())
			return nil
		}, nil

	case dstType == types.UUID && srcType == types.String:
		return func(dst, src unsafe.Pointer) error {
			x := *(*string)(src)
			u, err := uuid.Parse(x)
			if err != nil {
				return err
			}
			*(*uuid.UUID)(dst) = u
			return nil
		}, nil

	case srcType == types.ULID && dstType == types.String:
		return func(dst, src unsafe.Pointer) error {
			x := (*ulid.ULID)(src)
			*(*string)(dst) = x.String()
			return nil
		}, nil

	case dstType == types.ULID && srcType == types.String:
		return func(dst, src unsafe.Pointer) error {
			x := *(*string)(src)
			u, err := ulid.Parse(x)
			if err != nil {
				return err
			}
			*(*ulid.ULID)(dst) = u
			return nil
		}, nil

	case srcType == types.Decimal && dstType == types.String:
		return func(dst, src unsafe.Pointer) error {
			x := (*decimal.Decimal)(src)
			*(*string)(dst) = x.String()
			return nil
		}, nil

	case dstType == types.Decimal && srcType == types.String:
		return func(dst, src unsafe.Pointer) error {
			x := *(*string)(src)
			if x != "" {
				d, err := decimal.NewFromString(x)
				if err != nil {
					return err
				}
				*(*decimal.Decimal)(dst) = d
			}
			return nil
		}, nil

	case srcType == types.LanguageTag && dstType == types.String:
		return func(dst, src unsafe.Pointer) error {
			x := (*language.Tag)(src)
			*(*string)(dst) = x.String()
			return nil
		}, nil

	case dstType == types.LanguageTag && srcType == types.String:
		return func(dst, src unsafe.Pointer) error {
			x := *(*string)(src)
			t, err := language.Parse(x)
			if err != nil {
				return err
			}
			*(*language.Tag)(dst) = t
			return nil
		}, nil

	case srcType.Implements(types.Copiable):
		if !reflect.Zero(srcType).Interface().(iface.Copiable).CanCopyTo(dstType) {
			return nil, serr.New("can't copy", serr.String("srcType", srcType.Name()), serr.String("dstType", dstType.Name()))
		}
		dstSize := dstType.Size()
		return func(dst, src unsafe.Pointer) error {
			ptr := reflect.NewAt(srcType.Elem(), *(*unsafe.Pointer)(src)).Interface().(iface.Copiable).Copy(dstType)
			copy(
				unsafe.Slice((*byte)(dst), dstSize),
				unsafe.Slice((*byte)(ptr), dstSize),
			)
			return nil
		}, nil

	case srcType.Kind() == reflect.Pointer && dstType.Kind() == reflect.Pointer:
		dstElType, srcElType := dstType.Elem(), srcType.Elem()
		if dstElType == srcElType {
			return func(dst, src unsafe.Pointer) error {
				*(*unsafe.Pointer)(dst) = *(*unsafe.Pointer)(src)
				return nil
			}, nil
		}
		elConv, err := valConv(dstElType, srcElType)
		if err != nil {
			return nil, err
		}
		return func(dst, src unsafe.Pointer) error {
			if p := *(*unsafe.Pointer)(src); p != nil {
				newPtr := reflect.New(dstElType).UnsafePointer()
				if err := elConv(newPtr, p); err != nil {
					return err
				}
				*(*unsafe.Pointer)(dst) = newPtr
			}
			return nil
		}, nil

	case srcType.Kind() == reflect.Slice && dstType.Kind() == reflect.Slice:
		dstElType, srcElType := dstType.Elem(), srcType.Elem()
		elConv, err := valConv(dstElType, srcElType)
		if err != nil {
			return nil, err
		}
		dstElSize := dstElType.Size()
		srcElSize := srcElType.Size()
		return func(dst, src unsafe.Pointer) error {
			srcSlice := reflect.NewAt(srcType, src).Elem()
			if srcSlice.IsNil() {
				return nil
			}
			len := srcSlice.Len()
			dstSlice := reflect.MakeSlice(dstType, len, len)
			srcPtr := srcSlice.UnsafePointer()
			dstPtr := dstSlice.UnsafePointer()
			for i := 0; i < len; i++ {
				if err := elConv(dstPtr, srcPtr); err != nil {
					return err
				}
				dstPtr = unsafe.Add(dstPtr, dstElSize)
				srcPtr = unsafe.Add(srcPtr, srcElSize)
			}
			reflect.NewAt(dstType, dst).Elem().Set(dstSlice)
			return nil
		}, nil

	case srcPtrType.Implements(types.Maybe) && dstType.Kind() == reflect.Pointer:
		maybeType := reflect.Zero(reflect.PointerTo(srcType)).Interface().(maybe.Iface).MaybeType()
		if dstType == types.TimestampPtr && maybeType == types.Time {
			return func(dst, src unsafe.Pointer) error {
				x := reflect.NewAt(srcType, src).Interface().(maybe.Iface)
				if x := x.GetPtr(); x != nil {
					x := *(*time.Time)(x)
					*(**timestamppb.Timestamp)(dst) = timestamppb.New(x)
				}
				return nil
			}, nil
		}
		conv, err := valConv(dstType.Elem(), maybeType)
		if err != nil {
			return nil, err
		}
		return func(dst, src unsafe.Pointer) error {
			x := reflect.NewAt(srcType, src).Interface().(maybe.Iface)
			if x := x.GetPtr(); x != nil {
				v := reflect.New(dstType.Elem())
				if err := conv(v.UnsafePointer(), x); err != nil {
					return err
				}
				*(*unsafe.Pointer)(dst) = v.UnsafePointer()
			}
			return nil
		}, nil

	case dstPtrType.Implements(types.Maybe) && srcType.Kind() == reflect.Pointer:
		maybeType := reflect.Zero(dstPtrType).Interface().(maybe.Iface).MaybeType()
		if srcType == types.TimestampPtr && maybeType == types.Time {
			return func(dst, src unsafe.Pointer) error {
				x := *(**timestamppb.Timestamp)(src)
				if x.IsValid() {
					y := reflect.NewAt(dstType, dst).Interface().(maybe.Iface)
					y.SetPtr(unsafe.Pointer(pointer.To(x.AsTime())))
				}
				return nil
			}, nil
		}
		if srcType == types.TimestampPtr && maybeType == types.Date {
			return func(dst, src unsafe.Pointer) error {
				x := *(**timestamppb.Timestamp)(src)
				if x.IsValid() {
					y := reflect.NewAt(dstType, dst).Interface().(maybe.Iface)
					y.SetPtr(unsafe.Pointer(pointer.To(date.NewAt(x.AsTime()))))
				}
				return nil
			}, nil
		}
		conv, err := valConv(maybeType, srcType.Elem())
		if err != nil {
			return nil, err
		}
		return func(dst, src unsafe.Pointer) error {
			if p := *(*unsafe.Pointer)(src); p != nil {
				v := reflect.New(maybeType)
				if err := conv(v.UnsafePointer(), p); err != nil {
					return err
				}
				y := reflect.NewAt(dstType, dst).Interface().(maybe.Iface)
				y.SetPtr(v.UnsafePointer())
			}
			return nil
		}, nil

	case dstPtrType.Implements(types.Maybe) && srcType.Kind() != reflect.Pointer:
		maybeType := reflect.Zero(dstPtrType).Interface().(maybe.Iface).MaybeType()
		conv, err := valConv(maybeType, srcType)
		if err != nil {
			return nil, err
		}
		return func(dst, src unsafe.Pointer) error {
			if !reflect.NewAt(srcType, src).Elem().IsZero() {
				v := reflect.New(maybeType)
				if err := conv(v.UnsafePointer(), src); err != nil {
					return err
				}
				y := reflect.NewAt(dstType, dst).Interface().(maybe.Iface)
				y.SetPtr(v.UnsafePointer())
			}
			return nil
		}, nil

	case srcPtrType.Implements(types.Required):
		reqType := reflect.Zero(srcPtrType).Interface().(validate.RequiredIface).RequiredType()
		conv, err := valConv(dstType, reqType)
		if err != nil {
			return nil, err
		}
		return func(dst, src unsafe.Pointer) error {
			x := reflect.NewAt(srcType, src).Interface().(validate.RequiredIface)
			if !x.HasValue() {
				return serr.New("required field has no value", serr.String("field", srcType.Name()))

			}
			return conv(dst, x.UnsafePtr())
		}, nil

	case dstType.Kind() == reflect.Pointer:
		conv, err := valConv(dstType.Elem(), srcType)
		if err != nil {
			return nil, err
		}
		return func(dst, src unsafe.Pointer) error {
			v := reflect.New(dstType.Elem())
			if err := conv(v.UnsafePointer(), src); err != nil {
				return nil
			}
			*(*unsafe.Pointer)(dst) = v.UnsafePointer()
			return nil
		}, nil

	case srcType.Kind() == reflect.Pointer:
		conv, err := valConv(dstType, srcType.Elem())
		if err != nil {
			return nil, err
		}
		return func(dst, src unsafe.Pointer) error {
			if x := *(*unsafe.Pointer)(src); x != nil {
				return conv(dst, x)
			}
			return nil
		}, nil

	case dstType.Kind() == reflect.Struct && srcType.Kind() == reflect.Struct:
		copier, err := CopierForPair(dstType, srcType)
		if err != nil {
			return nil, err
		}
		return func(dst, src unsafe.Pointer) error {
			return copier(dst, src)
		}, nil

	case dstType == dynmapType && srcType.Kind() == reflect.Struct:
		fm := make(map[string][]int)
		for _, f := range reflect.VisibleFields(srcType) {
			if f.PkgPath != "" {
				continue
			}
			if f.Tag.Get("kv") == "-" {
				continue
			}
			key := f.Name
			if k := f.Tag.Get("key"); k != "" {
				key = k
			}
			fm[key] = f.Index
		}
		return func(dst, src unsafe.Pointer) error {
			mv := reflect.NewAt(dstType, dst).Elem()
			if mv.IsZero() {
				mv.Set(reflect.MakeMap(dstType))
			}
			m := mv.Interface().(map[string]interface{})
			v := reflect.NewAt(srcType, src).Elem()
			for k, idx := range fm {
				m[k] = v.FieldByIndex(idx).Interface()
			}
			return nil
		}, nil

	case srcType == dynmapType && dstType.Kind() == reflect.Struct:
		fm := make(map[string][]int)
		for _, f := range reflect.VisibleFields(dstType) {
			if f.PkgPath != "" {
				continue
			}
			if f.Tag.Get("kv") == "-" {
				continue
			}
			key := f.Name
			if k := f.Tag.Get("key"); k != "" {
				key = k
			}
			fm[key] = f.Index
		}
		return func(dst, src unsafe.Pointer) error {
			s := reflect.NewAt(dstType, dst).Elem()
			mv := reflect.NewAt(srcType, src).Elem()
			if mv.IsZero() {
				mv.Set(reflect.MakeMap(srcType))
			}
			m := mv.Interface().(map[string]interface{})
			for k, idx := range fm {
				x, ok := m[k]
				if !ok {
					return serr.New("missing field in structure for key", serr.String("key", k))
				}
				v := reflect.ValueOf(x)
				f := s.FieldByIndex(idx)
				if !v.Type().AssignableTo(f.Type()) {
					return serr.New("unable to set field in structure for key", serr.String("key", k))
				}
				f.Set(v)
			}
			return nil
		}, nil

	default:
		return nil, serr.New("don't know how to copy value", serr.String("srcType", srcType.Name()), serr.String("dstType", dstType.Name()))
	}
}

func fieldCopier(dstType, srcType reflect.Type, dstOffset, srcOffset uintptr) (func(unsafe.Pointer, unsafe.Pointer) error, error) {
	conv, err := valConv(dstType, srcType)
	if err != nil {
		return nil, err
	}
	return func(dst, src unsafe.Pointer) error {
		dst = unsafe.Add(dst, dstOffset)
		src = unsafe.Add(src, srcOffset)
		return conv(dst, src)
	}, nil
}

// NewCopy copies the contents of the source object to the destination object.
func NewCopy(dst, src interface{}) error {
	dstVal := reflect.ValueOf(dst)
	srcVal := reflect.ValueOf(src)
	dstType := dstVal.Type()
	srcType := srcVal.Type()
	dstTypeElem := dstType.Elem()
	srcTypeElem := srcType.Elem()
	if dstType.Kind() != reflect.Pointer || dstTypeElem.Kind() != reflect.Struct ||
		srcType.Kind() != reflect.Pointer || srcTypeElem.Kind() != reflect.Struct {
		return serr.Wrap("", ErrUnsupportedTypePair, serr.String("srcTypeElem", srcTypeElem.Name()), serr.String("dstTypeElem", dstTypeElem.Name()))
	}
	copier, err := CopierForPair(dstTypeElem, srcTypeElem)
	if err != nil {
		return err
	}
	return copier(dstVal.UnsafePointer(), srcVal.UnsafePointer())
}

// NewerCopy copies the contents of the source object to the destination object.
func NewerCopy[T, U any](dst *T, src *U) error {
	copier, err := CopierForPair(reflect.TypeFor[T](), reflect.TypeFor[U]())
	if err != nil {
		return err
	}
	return copier(unsafe.Pointer(dst), unsafe.Pointer(src))
}

// TypedCopierForPair creates a typed copier for a pair of structs.
func TypedCopierForPair[D, S any]() (func(*D, *S) error, error) {
	c, err := CopierForPairWithOptions(reflect.TypeFor[D](), reflect.TypeFor[S](), nil)
	if err != nil {
		return nil, err
	}
	return func(dst *D, src *S) error {
		return c(unsafe.Pointer(dst), unsafe.Pointer(src))
	}, nil
}

// SliceCopierForPair creates a typed copier for a pair of slices.
func SliceCopierForPair[D, S any]() (func([]*S) ([]*D, error), error) {
	c, err := TypedCopierForPair[D, S]()
	if err != nil {
		return nil, err
	}
	return func(src []*S) ([]*D, error) {
		r := make([]*D, 0, len(src))
		for _, x := range src {
			var y D
			if err := c(&y, x); err != nil {
				return nil, err
			}
			r = append(r, &y)
		}
		return r, nil
	}, nil
}

// MustSliceCopierForPair creates a typed copier for a pair of slices. It panics on error.
func MustSliceCopierForPair[D, S any]() func([]*S) ([]*D, error) {
	return must.Must(SliceCopierForPair[D, S]())
}

// CopyMap creates a copy of the given slice.
// Types T and U have to be compatible.
// [Copy] is used for mapping between the two types.
func CopyMap[T, U any](l []*T) ([]*U, error) {
	return slice.FallibleFmap(func(x *T) (*U, error) {
		var r U
		if err := Copy(&r, x); err != nil {
			return nil, err
		}
		return &r, nil
	}, l)
}
