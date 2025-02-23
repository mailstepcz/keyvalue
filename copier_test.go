package keyvalue

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"
	"unsafe"

	"github.com/google/uuid"
	"github.com/mailstepcz/enums"
	"github.com/mailstepcz/maybe"
	"github.com/mailstepcz/pointer"
	"github.com/mailstepcz/validate"
	"github.com/oklog/ulid/v2"
	"github.com/rickb777/date/v2"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
	"google.golang.org/protobuf/types/known/timestamppb"
	jinzhu "gopkg.in/jinzhu/copier.v0"
)

var abcEnum = enums.NewClosedEnum("a1", "b2", "c3", "d4")

type AbcEnum enums.String

func (v AbcEnum) EnumValueIsValid() bool {
	_, ok := enums.EnumGet[AbcEnum](&abcEnum, string(v))
	return ok
}
func (v AbcEnum) Value() (driver.Value, error) {
	return string(v), nil
}

func (v AbcEnum) DefaultValue() string {
	return string(abcEnum.DefaultValue())
}

type enumDst struct {
	X AbcEnum
	Y string
	U maybe.Maybe[AbcEnum]
	V *string
}

type enumSrc struct {
	X string
	Y AbcEnum
	U *string
	V maybe.Maybe[AbcEnum]
}

func TestClosedEnumCopySuccess(t *testing.T) {
	req := require.New(t)

	var dst enumDst
	u := "c3"
	err := Copy(&dst, &enumSrc{
		X: "a1",
		Y: AbcEnum("b2"),
		U: &u,
		V: maybe.Unit(AbcEnum("d4")),
	})
	req.Nil(err)
	req.Equal(AbcEnum("a1"), dst.X)
	req.Equal("b2", dst.Y)
	req.Equal(maybe.Unit(AbcEnum("c3")), dst.U)
	req.Equal("d4", *dst.V)
}

func TestClosedEnumCopyFailure(t *testing.T) {
	req := require.New(t)

	var dst enumDst
	err := Copy(&dst, &enumSrc{X: "aa11"})
	req.NotNil(err)
	req.Equal("bad value for closed enum value=aa11 dstType=AbcEnum", err.Error())
}

type embeddedDst struct {
	S string
	U string
}

type embeddedSrc struct {
	S string
	U uuid.UUID
}

func (x *embeddedSrc) toDst() *embeddedDst {
	return &embeddedDst{
		S: x.S,
		U: x.U.String(),
	}
}

type copierDst1 struct {
	N     int
	S     string
	X     float64
	ST2   *string
	SP1   *string
	SP2   string
	SP3   string
	UUID1 string
	UUID2 uuid.UUID
	UUID3 *uuid.UUID
	UUID4 *uuid.UUID
	TS1   time.Time
	TS2   *timestamppb.Timestamp
	TS3   *time.Time
	TS4   *timestamppb.Timestamp
	SL1   []uuid.UUID
	I1    embeddedDst
	I2    *embeddedDst
	I3    *embeddedDst
	S2    string
	S3    []byte
	D1    string
	D2    decimal.Decimal
	L1    language.Tag
	MB1   maybe.Maybe[string]
	MB2   *string
	MB3   maybe.Maybe[uuid.UUID]
	MB4   *uuid.UUID
	MB5   maybe.Maybe[uuid.UUID]
	MB6   *string
	MB7   maybe.Maybe[language.Tag]
	MB8   *string
	TS5   maybe.Maybe[time.Time]
	TS6   *timestamppb.Timestamp
	DM1   maybe.Maybe[decimal.Decimal]
	DM2   *string
	TS7   *timestamppb.Timestamp
	D3    date.Date
	D4    *timestamppb.Timestamp
	D5    maybe.Maybe[date.Date]
}

type copierSrc1 struct {
	N       int
	S       string
	X       float64
	ST2     string
	SP1     string
	SP2     *string
	SP3     *string
	UUID1   uuid.UUID
	UUID2   string
	UUID3   string
	UUID4   string
	TS1     *timestamppb.Timestamp
	TS2     time.Time
	TS3     *timestamppb.Timestamp
	TS4     *time.Time
	SL1     []string
	I1      embeddedSrc
	I2      *embeddedSrc
	I3      *embeddedSrc
	S2      []byte
	S3      string
	private int
	D1      decimal.Decimal
	D2      string
	L1      string
	MB1     *string
	MB2     maybe.Maybe[string]
	MB3     *uuid.UUID
	MB4     maybe.Maybe[uuid.UUID]
	MB5     *string
	MB6     maybe.Maybe[uuid.UUID]
	MB7     *string
	MB8     maybe.Maybe[language.Tag]
	TS5     *timestamppb.Timestamp
	TS6     maybe.Maybe[time.Time]
	DM1     *string
	DM2     maybe.Maybe[decimal.Decimal]
	TS7     maybe.Maybe[time.Time]
	D3      *timestamppb.Timestamp
	D4      date.Date
	D5      *timestamppb.Timestamp
}

func (x *copierSrc1) toDst() (*copierDst1, error) {
	u, err := uuid.Parse(x.UUID2)
	if err != nil {
		return nil, err
	}
	us := make([]uuid.UUID, len(x.SL1))
	for i, x := range x.SL1 {
		u, err := uuid.Parse(x)
		if err != nil {
			return nil, err
		}
		us[i] = u
	}
	var sp3 string
	if x.SP3 != nil {
		sp3 = *x.SP3
	}
	ts3 := x.TS3.AsTime()
	dec1 := x.D1.String()
	var uuid3 *uuid.UUID
	if x.UUID3 != "" && x.UUID3 != uuid.Nil.String() {
		z, err := uuid.Parse(x.UUID3)
		if err != nil {
			return nil, err
		}
		uuid3 = &z
	}
	uuid4, err := uuid.Parse(x.UUID4)
	if err != nil {
		return nil, err
	}
	dec2 := decimal.RequireFromString(x.D2)
	var st2 *string
	if x.ST2 != "" {
		st2 = &x.ST2
	}
	var mb1 maybe.Maybe[string]
	if x.MB1 != nil {
		mb1 = maybe.Unit(*x.MB1)
	}
	var mb2 *string
	if x.MB2.Valid {
		mb2 = &x.MB2.Val
	}
	var mb3 maybe.Maybe[uuid.UUID]
	if x.MB3 != nil {
		mb3 = maybe.Unit(*x.MB3)
	}
	var mb4 *uuid.UUID
	if x.MB4.Valid {
		mb4 = &x.MB4.Val
	}
	var mb5 maybe.Maybe[uuid.UUID]
	if x.MB5 != nil {
		mb5 = maybe.Unit(uuid.MustParse(*x.MB5))
	}
	var mb6 *string
	if x.MB6.Valid {
		u := x.MB6.Val.String()
		mb6 = &u
	}
	var mb7 maybe.Maybe[language.Tag]
	if x.MB7 != nil {
		mb7 = maybe.Unit(language.MustParse(*x.MB7))
	}
	var mb8 *string
	if x.MB8.Valid {
		l := x.MB8.Val.String()
		mb8 = &l
	}

	var ts5 maybe.Maybe[time.Time]
	if x.TS5.IsValid() {
		ts5 = maybe.Unit(x.TS5.AsTime())
	}
	var ts6 *timestamppb.Timestamp
	if x.TS6.Valid {
		ts6 = timestamppb.New(x.TS6.Val)
	}
	var dm1 maybe.Maybe[decimal.Decimal]
	if x.DM1 != nil {
		d, err := decimal.NewFromString(*x.DM1)
		if err != nil {
			return nil, err
		}
		dm1 = maybe.Unit(d)
	}
	var dm2 *string
	if x.DM2.Valid {
		s := x.DM2.Val.String()
		dm2 = &s
	}
	return &copierDst1{
		N:     x.N,
		S:     x.S,
		X:     x.X,
		ST2:   st2,
		SP1:   &x.SP1,
		SP2:   *x.SP2,
		SP3:   sp3,
		UUID1: x.UUID1.String(),
		UUID2: u,
		UUID3: uuid3,
		UUID4: &uuid4,
		TS1:   x.TS1.AsTime(),
		TS2:   timestamppb.New(x.TS2),
		TS3:   &ts3,
		TS4:   timestamppb.New(*x.TS4),
		SL1:   us,
		I1:    *x.I1.toDst(),
		I2:    x.I2.toDst(),
		I3:    nil,
		S2:    string(x.S2),
		S3:    []byte(x.S3),
		D1:    dec1,
		D2:    dec2,
		L1:    language.Afrikaans,
		MB1:   mb1,
		MB2:   mb2,
		MB3:   mb3,
		MB4:   mb4,
		MB5:   mb5,
		MB6:   mb6,
		MB7:   mb7,
		MB8:   mb8,
		TS5:   ts5,
		TS6:   ts6,
		DM1:   dm1,
		DM2:   dm2,
		D3:    date.NewAt(time.Now()),
		D4:    timestamppb.Now(),
	}, nil
}

type copierDst2 struct {
	N int
	S string
}

type copierSrc4 struct {
	N int
	S string
	X float64
}

type copierDst3 struct {
	N     int
	S     string
	X     float64
	UUID1 string
	UUID2 uuid.UUID
	TS1   time.Time
	TS2   *timestamppb.Timestamp
	SL1   []uuid.UUID
	S2    string
	S3    []byte
}

type copierSrc3 struct {
	N     int
	S     string
	X     float64
	UUID1 uuid.UUID
	UUID2 string
	TS1   *timestamppb.Timestamp
	TS2   time.Time
	SL1   []string
	S2    []byte
	S3    string
}

func (x *copierSrc3) toDst() (*copierDst3, error) {
	u, err := uuid.Parse(x.UUID2)
	if err != nil {
		return nil, err
	}
	us := make([]uuid.UUID, len(x.SL1))
	for i, x := range x.SL1 {
		u, err := uuid.Parse(x)
		if err != nil {
			return nil, err
		}
		us[i] = u
	}
	return &copierDst3{
		N:     x.N,
		S:     x.S,
		X:     x.X,
		UUID1: x.UUID1.String(),
		UUID2: u,
		TS1:   x.TS1.AsTime(),
		TS2:   timestamppb.New(x.TS2),
		SL1:   us,
		S2:    string(x.S2),
		S3:    []byte(x.S3),
	}, nil
}

func BenchmarkOldCopy(b *testing.B) {
	var lr interface{}
	copiers = make(map[copierTypePair]func(unsafe.Pointer, unsafe.Pointer) error)
	uid1, uid2, uid3 := uuid.New(), uuid.New(), uuid.New()
	uids1, uids2, uids3 := uid1.String(), uid2.String(), uid3.String()
	tm1, tm2 := time.Unix(12345678, 0).UTC(), time.Unix(12345679, 0).UTC()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var dst copierDst3
		if err := CopyV1(&dst, &copierSrc3{
			N:     1234,
			S:     "text",
			X:     12.34,
			UUID1: uid1,
			UUID2: uids2,
			TS1:   timestamppb.New(tm1),
			TS2:   tm2,
			SL1:   []string{uids1, uids2, uids3},
			S2:    []byte("S2 text"),
			S3:    "S3 text",
		}); err != nil {
			b.Errorf("copying failed: %v", err)
		}
		lr = &dst
	}
	gr = lr
}

func BenchmarkNewCopy(b *testing.B) {
	var lr interface{}
	copiers = make(map[copierTypePair]func(unsafe.Pointer, unsafe.Pointer) error)
	if _, err := CopierForPair(reflect.TypeOf((*copierDst3)(nil)).Elem(), reflect.TypeOf((*copierSrc3)(nil)).Elem()); err != nil {
		b.Errorf("copying failed: %v", err)
	}
	uid1, uid2, uid3 := uuid.New(), uuid.New(), uuid.New()
	uids1, uids2, uids3 := uid1.String(), uid2.String(), uid3.String()
	tm1, tm2 := time.Unix(12345678, 0).UTC(), time.Unix(12345679, 0).UTC()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var dst copierDst3
		if err := NewCopy(&dst, &copierSrc3{
			N:     1234,
			S:     "text",
			X:     12.34,
			UUID1: uid1,
			UUID2: uids2,
			TS1:   timestamppb.New(tm1),
			TS2:   tm2,
			SL1:   []string{uids1, uids2, uids3},
			S2:    []byte("S2 text"),
			S3:    "S3 text",
		}); err != nil {
			b.Errorf("copying failed: %v", err)
		}
		lr = &dst
	}
	gr = lr
}

func BenchmarkNewerCopy(b *testing.B) {
	var lr interface{}
	copiers = make(map[copierTypePair]func(unsafe.Pointer, unsafe.Pointer) error)
	if _, err := CopierForPair(reflect.TypeOf((*copierDst3)(nil)).Elem(), reflect.TypeOf((*copierSrc3)(nil)).Elem()); err != nil {
		b.Errorf("copying failed: %v", err)
	}
	uid1, uid2, uid3 := uuid.New(), uuid.New(), uuid.New()
	uids1, uids2, uids3 := uid1.String(), uid2.String(), uid3.String()
	tm1, tm2 := time.Unix(12345678, 0).UTC(), time.Unix(12345679, 0).UTC()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var dst copierDst3
		if err := NewerCopy(&dst, &copierSrc3{
			N:     1234,
			S:     "text",
			X:     12.34,
			UUID1: uid1,
			UUID2: uids2,
			TS1:   timestamppb.New(tm1),
			TS2:   tm2,
			SL1:   []string{uids1, uids2, uids3},
			S2:    []byte("S2 text"),
			S3:    "S3 text",
		}); err != nil {
			b.Errorf("copying failed: %v", err)
		}
		lr = &dst
	}
	gr = lr
}

func BenchmarkTypedCopy(b *testing.B) {
	var lr interface{}
	copiers = make(map[copierTypePair]func(unsafe.Pointer, unsafe.Pointer) error)
	copier, err := CopierForPair(reflect.TypeOf((*copierDst3)(nil)).Elem(), reflect.TypeOf((*copierSrc3)(nil)).Elem())
	if err != nil {
		b.Errorf("copying failed: %v", err)
	}
	copier2 := func(dst *copierDst3, src *copierSrc3) error {
		return copier(unsafe.Pointer(dst), unsafe.Pointer(src))
	}
	uid1, uid2, uid3 := uuid.New(), uuid.New(), uuid.New()
	uids1, uids2, uids3 := uid1.String(), uid2.String(), uid3.String()
	tm1, tm2 := time.Unix(12345678, 0).UTC(), time.Unix(12345679, 0).UTC()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var dst copierDst3
		if err := copier2(&dst, &copierSrc3{
			N:     1234,
			S:     "text",
			X:     12.34,
			UUID1: uid1,
			UUID2: uids2,
			TS1:   timestamppb.New(tm1),
			TS2:   tm2,
			SL1:   []string{uids1, uids2, uids3},
			S2:    []byte("S2 text"),
			S3:    "S3 text",
		}); err != nil {
			b.Errorf("copying failed: %v", err)
		}
		lr = &dst
	}
	gr = lr
}

func BenchmarkTypedReflectionCopy(b *testing.B) {
	var lr interface{}
	copiers = make(map[copierTypePair]func(unsafe.Pointer, unsafe.Pointer) error)
	copier, err := CopierForPair(reflect.TypeOf((*copierDst3)(nil)).Elem(), reflect.TypeOf((*copierSrc3)(nil)).Elem())
	if err != nil {
		b.Errorf("copying failed: %v", err)
	}
	var copier2 func(*copierDst3, *copierSrc3) error
	f := reflect.ValueOf(&copier2).Elem()
	errorType := reflect.TypeFor[error]()
	nilErrorRetSlice := []reflect.Value{reflect.Zero(errorType)}
	f.Set(reflect.MakeFunc(f.Type(), func(in []reflect.Value) []reflect.Value {
		if err := copier(in[0].UnsafePointer(), in[1].UnsafePointer()); err != nil {
			return []reflect.Value{reflect.ValueOf(err)}
		}
		return nilErrorRetSlice
	}))
	uid1, uid2, uid3 := uuid.New(), uuid.New(), uuid.New()
	uids1, uids2, uids3 := uid1.String(), uid2.String(), uid3.String()
	tm1, tm2 := time.Unix(12345678, 0).UTC(), time.Unix(12345679, 0).UTC()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var dst copierDst3
		if err := copier2(&dst, &copierSrc3{
			N:     1234,
			S:     "text",
			X:     12.34,
			UUID1: uid1,
			UUID2: uids2,
			TS1:   timestamppb.New(tm1),
			TS2:   tm2,
			SL1:   []string{uids1, uids2, uids3},
			S2:    []byte("S2 text"),
			S3:    "S3 text",
		}); err != nil {
			b.Errorf("copying failed: %v", err)
		}
		lr = &dst
	}
	gr = lr
}

func BenchmarkJinzhuCopy(b *testing.B) {
	var lr interface{}
	copiers = make(map[copierTypePair]func(unsafe.Pointer, unsafe.Pointer) error)
	uid1, uid2, uid3 := uuid.New(), uuid.New(), uuid.New()
	uids1, uids2, uids3 := uid1.String(), uid2.String(), uid3.String()
	tm1, tm2 := time.Unix(12345678, 0).UTC(), time.Unix(12345679, 0).UTC()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var dst copierDst3
		if err := jinzhu.Copy(&dst, &copierSrc3{
			N:     1234,
			S:     "text",
			X:     12.34,
			UUID1: uid1,
			UUID2: uids2,
			TS1:   timestamppb.New(tm1),
			TS2:   tm2,
			SL1:   []string{uids1, uids2, uids3},
			S2:    []byte("S2 text"),
			S3:    "S3 text",
		}); err != nil {
			b.Errorf("copying failed: %v", err)
		}
		lr = &dst
	}
	gr = lr
}

func BenchmarkNativeCopy(b *testing.B) {
	var lr interface{}
	uid1, uid2, uid3 := uuid.New(), uuid.New(), uuid.New()
	uids1, uids2, uids3 := uid1.String(), uid2.String(), uid3.String()
	tm1, tm2 := time.Unix(12345678, 0).UTC(), time.Unix(12345679, 0).UTC()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst, err := (&copierSrc3{
			N:     1234,
			S:     "text",
			X:     12.34,
			UUID1: uid1,
			UUID2: uids2,
			TS1:   timestamppb.New(tm1),
			TS2:   tm2,
			SL1:   []string{uids1, uids2, uids3},
			S2:    []byte("S2 text"),
			S3:    "S3 text",
		}).toDst()
		if err != nil {
			b.Errorf("copying failed: %v", err)
		}
		lr = &dst
	}
	gr = lr
}

type reqSrc struct {
	N     validate.Required[int]
	S     validate.Required[string]
	T     validate.Required[time.Time]
	UUID  validate.Required[uuid.UUID]
	UUID2 validate.Required[uuid.UUID]
}

type reqDst struct {
	N     int
	S     string
	T     *timestamppb.Timestamp
	UUID  string
	UUID2 string
}

func TestCopierRequired(t *testing.T) {
	req := require.New(t)

	copiers = make(map[copierTypePair]func(unsafe.Pointer, unsafe.Pointer) error)

	copier, err := CopierForPair(reflect.TypeFor[reqDst](), reflect.TypeFor[reqSrc]())
	req.Nil(err)

	var src reqSrc
	err = json.Unmarshal([]byte(`{"N":1234,"S":"abcd","T":"2023-12-31T05:00:00Z","UUID":"6c197756-0c3b-449c-8913-d2bc03ae9afd"}`), &src)
	req.Nil(err)

	var dst reqDst
	err = copier(unsafe.Pointer(&dst), unsafe.Pointer(&src))
	req.ErrorContains(err, "required field has no value field=Required[github.com/google/uuid.UUID]")

	req.Equal(1234, dst.N)
	req.Equal("abcd", dst.S)
	req.True(dst.T.IsValid())
	req.Equal("2023-12-31T05:00:00Z", dst.T.AsTime().Format(time.RFC3339))
	req.Equal(uuid.MustParse("6c197756-0c3b-449c-8913-d2bc03ae9afd").String(), dst.UUID)
}

type copOuterSrc struct {
	X *copInnerSrc
}

type copOuterDst struct {
	X *copInnerDst
}

type copInnerSrc struct {
	N int
}

type copInnerDst struct {
	S string
}

func (x *copInnerSrc) CanCopyTo(t reflect.Type) bool {
	return t == reflect.TypeFor[*copInnerDst]()
}

func (x *copInnerSrc) Copy(t reflect.Type) unsafe.Pointer {
	switch t {
	case reflect.TypeFor[*copInnerDst]():
		r := &copInnerDst{
			S: strconv.Itoa(x.N),
		}
		return unsafe.Pointer(&r)
	default:
		panic("can't copy type " + t.String())
	}
}

func TestCopierCopiable(t *testing.T) {
	req := require.New(t)

	copiers = make(map[copierTypePair]func(unsafe.Pointer, unsafe.Pointer) error)

	copier, err := CopierForPair(reflect.TypeFor[copOuterDst](), reflect.TypeFor[copOuterSrc]())
	req.Nil(err)

	var dst copOuterDst
	err = copier(unsafe.Pointer(&dst), unsafe.Pointer(&copOuterSrc{
		X: &copInnerSrc{
			N: 1234,
		},
	}))
	req.Nil(err)

	req.Equal("1234", dst.X.S)
}

func TestCopierCreationSuccess(t *testing.T) {
	req := require.New(t)

	copiers = make(map[copierTypePair]func(unsafe.Pointer, unsafe.Pointer) error)

	copier, err := CopierForPair(reflect.TypeOf((*copierDst1)(nil)).Elem(), reflect.TypeOf((*copierSrc1)(nil)).Elem())
	req.Nil(err)

	var dst copierDst1
	s := "another text pointer"
	uid1, uid2, uid3, uuid4 := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	tm1, tm2 := time.Unix(12345678, 0).UTC(), time.Unix(12345679, 0).UTC()
	tm3, tm4 := time.Unix(12345680, 0).UTC(), time.Unix(12345681, 0).UTC()
	tm5, tm6 := time.Unix(12345682, 0).UTC(), time.Unix(12345683, 0).UTC()
	mb1 := "MB1"
	mb3 := uuid.New()
	mb4 := uuid.New()
	mb5 := uuid.New()
	mb6 := uuid.New()
	mb7 := "cs"
	u5 := mb5.String()
	dm1 := "12.34"

	err = copier(unsafe.Pointer(&dst), unsafe.Pointer(&copierSrc1{
		N:     1234,
		S:     "text",
		ST2:   "",
		X:     12.34,
		SP1:   "text pointer",
		SP2:   &s,
		UUID1: uid1,
		UUID2: uid2.String(),
		UUID3: "",
		UUID4: uuid4.String(),
		TS1:   timestamppb.New(tm1),
		TS2:   tm2,
		TS3:   timestamppb.New(tm3),
		TS4:   &tm4,
		SL1:   []string{uid1.String(), uid2.String(), uid3.String()},
		I1: embeddedSrc{
			S: "abcd",
			U: uid1,
		},
		I2: &embeddedSrc{
			S: "efgh",
			U: uid2,
		},
		I3:  nil,
		S2:  []byte("S2 text"),
		S3:  "S3 text",
		L1:  "cs",
		MB1: &mb1,
		MB2: maybe.Unit("MB2"),
		MB3: &mb3,
		MB4: maybe.Unit(mb4),
		MB5: &u5,
		MB6: maybe.Unit(mb6),
		MB7: &mb7,
		MB8: maybe.Unit(language.Czech),
		TS5: timestamppb.New(tm5),
		TS6: maybe.Unit(tm6),
		DM1: &dm1,
		DM2: maybe.Unit(decimal.New(5678, -2)),
		TS7: maybe.Nothing[time.Time](),
		D3:  timestamppb.Now(),
		D4:  date.Today(),
		D5:  timestamppb.Now(),
	}))
	req.Nil(err)

	req.Equal(1234, dst.N)
	req.Equal("text", dst.S)
	req.Equal("", *dst.ST2)
	req.Equal(12.34, dst.X)
	req.Equal("text pointer", *dst.SP1)
	req.Equal("another text pointer", dst.SP2)
	req.Equal("", dst.SP3)
	req.Equal(uid1.String(), dst.UUID1)
	req.Equal(uid2, dst.UUID2)
	req.Nil(dst.UUID3)
	req.Equal(uuid4.String(), dst.UUID4.String())
	req.Equal(tm1.Format(time.RFC3339), dst.TS1.Format(time.RFC3339))
	req.Equal(tm2.Format(time.RFC3339), dst.TS2.AsTime().Format(time.RFC3339))
	req.Equal(tm3.Format(time.RFC3339), dst.TS3.Format(time.RFC3339))
	req.True(dst.TS4.IsValid())
	req.Equal(tm4.Format(time.RFC3339), dst.TS4.AsTime().Format(time.RFC3339))
	req.Equal([]uuid.UUID{uid1, uid2, uid3}, dst.SL1)
	req.Equal("abcd", dst.I1.S)
	req.Equal(uid1.String(), dst.I1.U)
	req.Equal("efgh", dst.I2.S)
	req.Equal(uid2.String(), dst.I2.U)
	req.Nil(dst.I3)
	req.Equal("S2 text", dst.S2)
	req.Equal([]byte("S3 text"), dst.S3)
	req.Equal(language.Czech, dst.L1)
	req.Equal(maybe.Unit("MB1"), dst.MB1)
	req.Equal("MB2", *dst.MB2)
	req.Equal(maybe.Unit(mb3), dst.MB3)
	req.Equal(mb4, *dst.MB4)
	req.Equal(maybe.Unit(mb5), dst.MB5)
	req.Equal(mb6.String(), *dst.MB6)
	req.Equal(maybe.Unit(language.MustParse(mb7)), dst.MB7)
	req.Equal("cs", *dst.MB8)
	req.Equal(tm5, dst.TS5.Val)
	req.Equal(tm6, dst.TS6.AsTime())
	req.Equal(decimal.New(1234, -2), dst.DM1.Val)
	req.Equal("56.78", *dst.DM2)
	req.Nil(dst.TS7)
	req.Equal(date.Today(), dst.D3)
	req.Equal(timestamppb.New(time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.UTC)), dst.D4)
	req.Equal(date.New(time.Now().Year(), time.Now().Month(), time.Now().Day()), dst.D5.Val)
}

func TestNativeCopyCreationSuccess(t *testing.T) {
	req := require.New(t)

	s := "another text pointer"
	uid1, uid2, uid3, uuid4 := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	tm1, tm2 := time.Unix(12345678, 0).UTC(), time.Unix(12345679, 0).UTC()
	tm3, tm4 := time.Unix(12345680, 0).UTC(), time.Unix(12345681, 0).UTC()
	dst, err := (&copierSrc1{
		N:     1234,
		S:     "text",
		ST2:   "",
		X:     12.34,
		SP1:   "text pointer",
		SP2:   &s,
		UUID1: uid1,
		UUID2: uid2.String(),
		UUID3: uuid.Nil.String(),
		UUID4: uuid4.String(),
		TS1:   timestamppb.New(tm1),
		TS2:   tm2,
		TS3:   timestamppb.New(tm3),
		TS4:   &tm4,
		SL1:   []string{uid1.String(), uid2.String(), uid3.String()},
		I1: embeddedSrc{
			S: "abcd",
			U: uid1,
		},
		I2: &embeddedSrc{
			S: "efgh",
			U: uid2,
		},
		I3: nil,
		S2: []byte("S2 text"),
		S3: "S3 text",
		D1: decimal.NewFromFloatWithExponent(124, -3),
		D2: "1.25",
	}).toDst()
	req.Nil(err)

	req.Equal(1234, dst.N)
	req.Equal("text", dst.S)
	req.Nil(dst.ST2)
	req.Equal(12.34, dst.X)
	req.Equal("text pointer", *dst.SP1)
	req.Equal("another text pointer", dst.SP2)
	req.Equal("", dst.SP3)
	req.Equal(uid1.String(), dst.UUID1)
	req.Equal(uid2, dst.UUID2)
	req.Nil(dst.UUID3)
	req.Equal(uuid4.String(), dst.UUID4.String())
	req.Equal(tm1.Format(time.RFC3339), dst.TS1.Format(time.RFC3339))
	req.Equal(tm2.Format(time.RFC3339), dst.TS2.AsTime().Format(time.RFC3339))
	req.Equal(tm3.Format(time.RFC3339), dst.TS3.Format(time.RFC3339))
	req.True(dst.TS4.IsValid())
	req.Equal(tm4.Format(time.RFC3339), dst.TS4.AsTime().Format(time.RFC3339))
	req.Equal([]uuid.UUID{uid1, uid2, uid3}, dst.SL1)
	req.Equal("abcd", dst.I1.S)
	req.Equal(uid1.String(), dst.I1.U)
	req.Equal("efgh", dst.I2.S)
	req.Equal(uid2.String(), dst.I2.U)
	req.Equal(uid2.String(), dst.I2.U)
	req.Nil(dst.I3)
	req.Equal("S2 text", dst.S2)
	req.Equal([]byte("S3 text"), dst.S3)
}

type ulidSrc struct {
	ULID1 ulid.ULID
	ULID2 string
	ULID3 maybe.Maybe[ulid.ULID]
	ULID4 maybe.Maybe[ulid.ULID]
	ULID5 *string
	ULID6 *string
}

type ulidDst struct {
	ULID1 string
	ULID2 ulid.ULID
	ULID3 *string
	ULID4 *string
	ULID5 maybe.Maybe[ulid.ULID]
	ULID6 maybe.Maybe[ulid.ULID]
}

func TestUlidCopy(t *testing.T) {
	req := require.New(t)
	ulid5 := ulid.Make().String()
	src := ulidSrc{
		ULID1: ulid.Make(),
		ULID2: ulid.Make().String(),
		ULID3: maybe.Unit(ulid.Make()),
		ULID4: maybe.Nothing[ulid.ULID](),
		ULID5: &ulid5,
		ULID6: nil,
	}
	var dst ulidDst
	err := Copy(&dst, &src)
	req.NoError(err)

	req.Equal(src.ULID1.String(), dst.ULID1)
	req.Equal(src.ULID2, dst.ULID2.String())
	req.Equal(src.ULID3.Val.String(), *dst.ULID3)
	req.Nil(dst.ULID4)
	req.Equal(ulid5, dst.ULID5.Val.String())
	req.False(dst.ULID6.Valid)
}

func TestCopierCreationErrFieldNotInDestination(t *testing.T) {
	req := require.New(t)

	copiers = make(map[copierTypePair]func(unsafe.Pointer, unsafe.Pointer) error)

	_, err := CopierForPair(reflect.TypeOf((*copierDst2)(nil)).Elem(), reflect.TypeOf((*copierSrc1)(nil)).Elem())
	req.NotNil(err)

	req.Equal("field not found srcField=X srcType=copierSrc1", err.Error())
}

func TestCopierCreationNotSubsumedSuccess(t *testing.T) {
	req := require.New(t)

	copiers = make(map[copierTypePair]func(unsafe.Pointer, unsafe.Pointer) error)

	copier, err := CopierForPairWithOptions(
		reflect.TypeOf((*copierDst2)(nil)).Elem(),
		reflect.TypeOf((*copierSrc1)(nil)).Elem(),
		&CopierOptions{
			OmitNotFound: true,
		})
	req.Nil(err)

	var dst copierDst2
	err = copier(unsafe.Pointer(&dst), unsafe.Pointer(&copierSrc1{
		N: 1234,
		S: "text",
		X: 12.34,
	}))
	req.Nil(err)

	req.Equal(1234, dst.N)
	req.Equal("text", dst.S)
}

func TestCopierCreationFieldsToOmitSuccess(t *testing.T) {
	req := require.New(t)

	copiers = make(map[copierTypePair]func(unsafe.Pointer, unsafe.Pointer) error)

	copier, err := CopierForPairWithOptions(
		reflect.TypeOf((*copierDst2)(nil)).Elem(),
		reflect.TypeOf((*copierSrc4)(nil)).Elem(),
		&CopierOptions{
			FieldsToOmit: []string{"X"},
		})
	req.Nil(err)

	var dst copierDst2
	err = copier(unsafe.Pointer(&dst), unsafe.Pointer(&copierSrc4{
		N: 1234,
		S: "text",
		X: 12.34,
	}))
	req.Nil(err)

	req.Equal(1234, dst.N)
	req.Equal("text", dst.S)
}

func TestCopierCreationFieldsToCopySuccess(t *testing.T) {
	req := require.New(t)

	copiers = make(map[copierTypePair]func(unsafe.Pointer, unsafe.Pointer) error)

	copier, err := CopierForPairWithOptions(
		reflect.TypeOf((*copierDst2)(nil)).Elem(),
		reflect.TypeOf((*copierSrc4)(nil)).Elem(),
		&CopierOptions{
			FieldsToCopy: []string{"N", "S"},
		})
	req.Nil(err)

	var dst copierDst2
	err = copier(unsafe.Pointer(&dst), unsafe.Pointer(&copierSrc4{
		N: 1234,
		S: "text",
		X: 12.34,
	}))
	req.Nil(err)

	req.Equal(1234, dst.N)
	req.Equal("text", dst.S)
}

func TestFieldConv(t *testing.T) {
	t.Run("Required[T] -> T", func(t *testing.T) {
		req := require.New(t)

		type srcS struct {
			N validate.Required[int]
		}
		type dstS struct {
			N int
		}

		copier, err := CopierForPair(reflect.TypeFor[dstS](), reflect.TypeFor[srcS]())
		req.NoError(err)

		var src srcS
		err = json.Unmarshal([]byte(`{"N":1234}`), &src)
		req.NoError(err)

		var dst dstS
		err = copier(unsafe.Pointer(&dst), unsafe.Pointer(&src))
		req.NoError(err)
		req.Equal(1234, dst.N)
	})
}

func TestValConv(t *testing.T) {
	t.Run("int -> int", func(t *testing.T) {
		req := require.New(t)

		var (
			dst int
			src int = 1234
		)
		f, err := valConv(reflect.TypeOf(dst), reflect.TypeOf(src))
		req.NoError(err)
		err = f(unsafe.Pointer(&dst), unsafe.Pointer(&src))
		req.NoError(err)
		req.Equal(src, dst)
	})

	t.Run("*int -> *int", func(t *testing.T) {
		req := require.New(t)

		var (
			dst *int
			src *int = pointer.To(1234)
		)
		f, err := valConv(reflect.TypeOf(dst), reflect.TypeOf(src))
		req.NoError(err)
		err = f(unsafe.Pointer(&dst), unsafe.Pointer(&src))
		req.NoError(err)
		req.Equal(*src, *dst)
	})

	t.Run("int -> ~int", func(t *testing.T) {
		req := require.New(t)

		type alias int
		var (
			dst alias
			src int = 1234
		)
		f, err := valConv(reflect.TypeOf(dst), reflect.TypeOf(src))
		req.NoError(err)
		err = f(unsafe.Pointer(&dst), unsafe.Pointer(&src))
		req.NoError(err)
		req.Equal(alias(src), dst)
	})

	t.Run("[]int -> []int", func(t *testing.T) {
		req := require.New(t)

		var (
			dst []int
			src = []int{12, 34, 56}
		)
		f, err := valConv(reflect.TypeOf(dst), reflect.TypeOf(src))
		req.NoError(err)
		err = f(unsafe.Pointer(&dst), unsafe.Pointer(&src))
		req.NoError(err)
		req.Equal(src, dst)
	})

	t.Run("[]int -> []~int", func(t *testing.T) {
		req := require.New(t)

		type alias int
		var (
			dst []alias
			src = []int{12, 34, 56}
		)
		f, err := valConv(reflect.TypeOf(dst), reflect.TypeOf(src))
		req.NoError(err)
		err = f(unsafe.Pointer(&dst), unsafe.Pointer(&src))
		req.NoError(err)
		req.Equal([]alias{12, 34, 56}, dst)
	})

	t.Run("*int -> Maybe[int]", func(t *testing.T) {
		req := require.New(t)

		var (
			dst maybe.Maybe[int]
			src = pointer.To(1234)
		)
		f, err := valConv(reflect.TypeOf(dst), reflect.TypeOf(src))
		req.NoError(err)
		err = f(unsafe.Pointer(&dst), unsafe.Pointer(&src))
		req.NoError(err)
		req.Equal(1234, dst.Val)
	})

	t.Run("*int -> Maybe[int] (nil)", func(t *testing.T) {
		req := require.New(t)

		var (
			dst maybe.Maybe[int]
			src *int
		)
		f, err := valConv(reflect.TypeOf(dst), reflect.TypeOf(src))
		req.NoError(err)
		err = f(unsafe.Pointer(&dst), unsafe.Pointer(&src))
		req.NoError(err)
		req.False(dst.Valid)
	})

	t.Run("Maybe[int] -> *int", func(t *testing.T) {
		req := require.New(t)

		var (
			dst *int
			src = maybe.Unit(1234)
		)
		f, err := valConv(reflect.TypeOf(dst), reflect.TypeOf(src))
		req.NoError(err)
		err = f(unsafe.Pointer(&dst), unsafe.Pointer(&src))
		req.NoError(err)
		req.Equal(1234, *dst)
	})

	t.Run("Maybe[int] -> *int (nil)", func(t *testing.T) {
		req := require.New(t)

		var (
			dst *int
			src = maybe.Nothing[int]()
		)
		f, err := valConv(reflect.TypeOf(dst), reflect.TypeOf(src))
		req.NoError(err)
		err = f(unsafe.Pointer(&dst), unsafe.Pointer(&src))
		req.NoError(err)
		req.Nil(dst)
	})

	t.Run("*string -> Maybe[ULID]", func(t *testing.T) {
		req := require.New(t)

		u := ulid.Make()
		var (
			dst maybe.Maybe[ulid.ULID]
			src *string = pointer.To(u.String())
		)
		f, err := valConv(reflect.TypeOf(dst), reflect.TypeOf(src))
		req.NoError(err)
		err = f(unsafe.Pointer(&dst), unsafe.Pointer(&src))
		req.NoError(err)
		req.Equal(maybe.Unit(u), dst)
	})

	t.Run("*string -> Maybe[Decimal]", func(t *testing.T) {
		req := require.New(t)

		var (
			dst maybe.Maybe[decimal.Decimal]
			src *string = pointer.To("1234")
		)
		f, err := valConv(reflect.TypeOf(dst), reflect.TypeOf(src))
		req.NoError(err)
		err = f(unsafe.Pointer(&dst), unsafe.Pointer(&src))
		req.NoError(err)
		req.Equal(maybe.Unit(decimal.NewFromInt(1234)), dst)
	})

	t.Run("Maybe[Decimal] -> *string", func(t *testing.T) {
		req := require.New(t)

		var (
			dst *string
			src = maybe.Unit(decimal.NewFromInt(1234))
		)
		f, err := valConv(reflect.TypeOf(dst), reflect.TypeOf(src))
		req.NoError(err)
		err = f(unsafe.Pointer(&dst), unsafe.Pointer(&src))
		req.NoError(err)
		req.Equal("1234", *dst)
	})

	t.Run("struct -> struct", func(t *testing.T) {
		req := require.New(t)

		type dstS struct {
			X *timestamppb.Timestamp
			Y uuid.UUID
		}
		type srcS struct {
			X time.Time
			Y string
		}

		tm := time.Unix(12, 34)
		u := uuid.New()
		var (
			dst dstS
			src = srcS{
				X: tm,
				Y: u.String(),
			}
		)
		f, err := valConv(reflect.TypeFor[dstS](), reflect.TypeFor[srcS]())
		req.NoError(err)
		err = f(unsafe.Pointer(&dst), unsafe.Pointer(&src))
		req.NoError(err)
		req.Equal(tm.UTC(), dst.X.AsTime().UTC())
		req.Equal(u, dst.Y)
	})

	t.Run("*struct -> *struct", func(t *testing.T) {
		req := require.New(t)

		type dstS struct {
			X *timestamppb.Timestamp
			Y uuid.UUID
		}
		type srcS struct {
			X time.Time
			Y string
		}

		tm := time.Unix(12, 34)
		u := uuid.New()
		var (
			dst *dstS
			src = &srcS{
				X: tm,
				Y: u.String(),
			}
		)
		f, err := valConv(reflect.TypeFor[*dstS](), reflect.TypeFor[*srcS]())
		req.NoError(err)
		err = f(unsafe.Pointer(&dst), unsafe.Pointer(&src))
		req.NoError(err)
		req.Equal(tm.UTC(), dst.X.AsTime().UTC())
		req.Equal(u, dst.Y)
	})
}

func TestSliceCopier(t *testing.T) {
	type D struct {
		ID uuid.UUID
	}
	type S struct {
		ID string
	}

	t.Run("success", func(t *testing.T) {
		req := require.New(t)

		copier, err := SliceCopierForPair[D, S]()
		req.NoError(err)

		u1, u2, u3 := uuid.New(), uuid.New(), uuid.New()
		src := []*S{&S{u1.String()}, &S{u2.String()}, &S{u3.String()}}
		dst, err := copier(src)
		req.NoError(err)
		req.Equal([]*D{&D{u1}, &D{u2}, &D{u3}}, dst)
	})

	t.Run("failure", func(t *testing.T) {
		req := require.New(t)

		copier, err := SliceCopierForPair[D, S]()
		req.NoError(err)

		u1, u2, u3 := uuid.New(), uuid.New(), uuid.New()
		src := []*S{&S{u1.String()}, &S{u2.String()}, &S{u3.String()}, &S{"uuid"}}
		_, err = copier(src)
		req.NotNil(err)
		req.Equal("invalid UUID length: 4", err.Error())
	})
}

type copyStruct struct {
	N int
	S string
}

func BenchmarkStructArrayCopy(b *testing.B) {
	src := copyStruct{N: 1234, S: "abcd"}
	var lr interface{}
	for i := 0; i < b.N; i++ {
		var dst copyStruct
		*(*[unsafe.Sizeof(src)]byte)(unsafe.Pointer(&dst)) = *(*[unsafe.Sizeof(src)]byte)(unsafe.Pointer(&src))
		lr = &dst
	}
	gr = lr
}

func BenchmarkStructSliceCopy(b *testing.B) {
	src := copyStruct{N: 1234, S: "abcd"}
	var lr interface{}
	size := unsafe.Sizeof(src)
	for i := 0; i < b.N; i++ {
		var dst copyStruct
		copy(unsafe.Slice((*byte)(unsafe.Pointer(&dst)), size), unsafe.Slice((*byte)(unsafe.Pointer(&src)), size))
		lr = &dst
	}
	gr = lr
}

func BenchmarkStructReflSet(b *testing.B) {
	src := copyStruct{N: 1234, S: "abcd"}
	var lr interface{}
	for i := 0; i < b.N; i++ {
		var dst copyStruct
		reflect.ValueOf(&dst).Elem().Set(reflect.ValueOf(src))
		lr = &dst
	}
	gr = lr
}

type sliceWrapperSrc struct {
	IDs []uuid.UUID
}

type sliceWrapperDst struct {
	IDs []string
}

func BenchmarkWrappedSliceCopy(b *testing.B) {
	src := sliceWrapperSrc{
		IDs: []uuid.UUID{
			uuid.New(),
			uuid.New(),
			uuid.New(),
			uuid.New(),
			uuid.New(),
		},
	}
	var lr interface{}
	for i := 0; i < b.N; i++ {
		var dst sliceWrapperDst
		if err := Copy(&dst, &src); err != nil {
			b.Fatal("failed copy")
		}
		lr = &dst
	}
	gr = lr
}

func TestMemcopy(t *testing.T) {
	req := require.New(t)

	srcSlice := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	dstSlice := make([]byte, 8)
	memcopy(unsafe.Pointer(unsafe.SliceData(dstSlice)), unsafe.Pointer(unsafe.SliceData(srcSlice)), 8)
	req.Equal([]byte{1, 2, 3, 4, 5, 6, 7, 8}, dstSlice)
}

func BenchmarkSlicedMemcopy(b *testing.B) {
	srcSlice := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	dstSlice := make([]byte, 8)
	dst := unsafe.Pointer(unsafe.SliceData(dstSlice))
	src := unsafe.Pointer(unsafe.SliceData(srcSlice))
	size := len(srcSlice)
	var lr interface{}
	for i := 0; i < b.N; i++ {
		copy(unsafe.Slice((*byte)(dst), size), unsafe.Slice((*byte)(src), size))
		lr = dst
	}
	gr = lr
}

func BenchmarkOptimisedMemcopy(b *testing.B) {
	srcSlice := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	dstSlice := make([]byte, 8)
	dst := unsafe.Pointer(unsafe.SliceData(dstSlice))
	src := unsafe.Pointer(unsafe.SliceData(srcSlice))
	size := uintptr(len(srcSlice))
	var lr interface{}
	for i := 0; i < b.N; i++ {
		memcopy(dst, src, size)
		lr = dst
	}
	gr = lr
}

func TestCopierInterface(t *testing.T) {
	t.Run("string -> UUID", func(t *testing.T) {
		req := require.New(t)

		c, err := ValueCopier[uuid.UUID, string]()
		req.NoError(err)
		src := uuid.NewString()
		dst, err := Cast(c).Copy(src)
		req.NoError(err)
		req.Equal(src, dst.String())
	})

	t.Run("*string -> *UUID", func(t *testing.T) {
		req := require.New(t)

		c, err := ValueCopier[*uuid.UUID, *string]()
		req.NoError(err)
		src := uuid.NewString()
		dst, err := Cast(c).Copy(&src)
		req.NoError(err)
		req.Equal(src, dst.String())
	})
}

func TestStructToMap(t *testing.T) {
	req := require.New(t)

	type d struct {
		X map[string]interface{}
	}
	type p struct {
		S string
		N int `key:"Num"`
	}
	type s struct {
		X p
	}

	src := s{X: p{S: "abcd", N: 1234}}
	var dst d
	err := Copy(&dst, &src)
	req.NoError(err)
	req.Equal(map[string]interface{}{"S": "abcd", "Num": 1234}, dst.X)
}

func TestStructFromMap(t *testing.T) {
	req := require.New(t)

	type p struct {
		S string
		N int `key:"Num"`
	}
	type d struct {
		X p
	}
	type s struct {
		X map[string]interface{}
	}

	src := s{X: map[string]interface{}{"S": "abcd", "Num": 1234}}
	var dst d
	err := Copy(&dst, &src)
	req.NoError(err)
	req.Equal(p{S: "abcd", N: 1234}, dst.X)
}

type struct1 struct {
	ID string
}

type struct2 struct {
	ID uuid.UUID
}

func TestCopyMap(t *testing.T) {
	req := require.New(t)
	s := []*struct1{{
		ID: uuid.NewString(),
	}}

	res, err := CopyMap[struct1, struct2](s)
	req.NoError(err)
	req.Len(res, 1)
	req.Equal(s[0].ID, res[0].ID.String())

}

func ExampleCopy() {
	type source struct {
		ID string
	}
	type target struct {
		ID uuid.UUID
	}
	var t target
	if err := Copy(&t, &source{
		ID: "faf5914d-0734-4d91-b486-e046ce197292",
	}); err != nil {
		panic(err)
	}
	fmt.Println(t)
	// Output: {faf5914d-0734-4d91-b486-e046ce197292}
}

func ExampleCopy_second() {
	type source struct {
		ID string
	}
	type target struct {
		ID uuid.UUID
	}
	var t target
	if err := Copy(&t, &source{
		ID: "",
	}); err != nil {
		fmt.Println("error:", err)
	} else {
		fmt.Println(t)
	}
	// Output: error: invalid UUID length: 0
}

func ExampleCopy_third() {
	type source struct {
		ID string
	}
	type target struct {
		ID uuid.UUID
	}
	var t target
	if err := Copy(&t, &source{
		ID: "faf5914d-0734-4d91-b486-e046ce19729g",
	}); err != nil {
		fmt.Println("error:", err)
	} else {
		fmt.Println(t)
	}
	// Output: error: invalid UUID format
}
