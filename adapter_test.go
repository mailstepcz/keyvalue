package keyvalue

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"
	stduuid "uuid"

	"github.com/google/uuid"
	"github.com/mailstepcz/maybe"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Str string

type s1 struct {
	A                 int `kv:"-"`
	B                 int
	C                 float64
	D                 string
	T1                time.Time
	T2                *timestamppb.Timestamp
	U1                uuid.UUID
	U2                string
	X1                []string
	ZeroStringToMaybe string
	StringToMaybe     string
	ZeroIntToMaybe    int
	IntToMaybe        int
	ZeroUUIDToMaybe   uuid.UUID
	UUIDToMaybe       uuid.UUID
	ZeroTimeToMaybe   time.Time
	TimeToMaybe       time.Time
}

type s2 struct {
	A                 uuid.UUID
	B                 int
	C                 float64
	D                 Str
	T1                *timestamppb.Timestamp
	T2                time.Time
	U1                string
	U2                uuid.UUID
	X1                []interface{}
	ZeroStringToMaybe maybe.Maybe[string]
	StringToMaybe     maybe.Maybe[string]
	ZeroIntToMaybe    maybe.Maybe[int]
	IntToMaybe        maybe.Maybe[int]
	ZeroUUIDToMaybe   maybe.Maybe[uuid.UUID]
	UUIDToMaybe       maybe.Maybe[uuid.UUID]
	ZeroTimeToMaybe   maybe.Maybe[time.Time]
	TimeToMaybe       maybe.Maybe[time.Time]
}

func TestStructToStructAdapters(t *testing.T) {
	req := require.New(t)
	tm := time.Unix(12345678, 0).UTC()
	u := uuid.New()
	now := time.Now()
	s1 := s1{
		B:                 1234,
		C:                 12.34,
		D:                 "abcd",
		T1:                tm,
		T2:                timestamppb.New(tm),
		U1:                u,
		U2:                u.String(),
		X1:                []string{"foo", "bar", "baz"},
		ZeroStringToMaybe: "",
		StringToMaybe:     "something",
		ZeroIntToMaybe:    0,
		IntToMaybe:        10,
		ZeroUUIDToMaybe:   uuid.Nil,
		UUIDToMaybe:       uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		ZeroTimeToMaybe:   time.Time{},
		TimeToMaybe:       now,
	}
	var s2 s2
	err := NewCopy(&s2, &s1)
	req.Nil(err)
	req.Equal(s1.B, s2.B)
	req.Equal(s1.C, s2.C)
	req.Equal(s1.D, string(s2.D))
	req.Equal(s1.T1, s2.T1.AsTime())
	req.Equal(s1.T2.AsTime(), s2.T2)
	req.Equal(s1.U1.String(), s2.U1)
	req.Equal(s1.U2, s2.U2.String())
	req.Equal("bar", s2.X1[1])
	req.Equal(maybe.Nothing[string](), s2.ZeroStringToMaybe)
	req.False(s2.ZeroStringToMaybe.Valid)
	req.Equal(maybe.Unit("something"), s2.StringToMaybe)
	req.True(s2.StringToMaybe.Valid)
	req.Equal(maybe.Nothing[int](), s2.ZeroIntToMaybe)
	req.False(s2.ZeroIntToMaybe.Valid)
	req.Equal(maybe.Unit(10), s2.IntToMaybe)
	req.True(s2.IntToMaybe.Valid)
	req.Equal(maybe.Nothing[uuid.UUID](), s2.ZeroUUIDToMaybe)
	req.False(s2.ZeroUUIDToMaybe.Valid)
	req.Equal(maybe.Unit(uuid.MustParse("00000000-0000-0000-0000-000000000001")), s2.UUIDToMaybe)
	req.True(s2.UUIDToMaybe.Valid)
	req.Equal(maybe.Nothing[time.Time](), s2.ZeroTimeToMaybe)
	req.False(s2.ZeroTimeToMaybe.Valid)
	req.Equal(maybe.Unit(now), s2.TimeToMaybe)
	req.True(s2.TimeToMaybe.Valid)
}

func TestStructToMapAdapters(t *testing.T) {
	req := require.New(t)
	tm := time.Unix(12345678, 0).UTC()
	u := uuid.New()
	s1 := s1{
		B:  1234,
		C:  12.34,
		D:  "abcd",
		T1: tm,
		T2: timestamppb.New(tm),
		U1: u,
		U2: u.String(),
	}
	m := make(map[string]interface{})
	err := CopyV1(m, &s1)
	req.Nil(err)
	req.Equal(s1.B, m["B"])
	req.Equal(s1.C, m["C"])
	req.Equal(s1.D, m["D"])
	req.Equal(s1.T1, m["T1"])
	req.Equal(s1.T2, m["T2"])
	req.Equal(s1.U1, m["U1"])
	req.Equal(s1.U2, m["U2"])
}

func TestMapToStructAdapters(t *testing.T) {
	req := require.New(t)
	tm := time.Unix(12345678, 0).UTC()
	u := uuid.New()
	m := map[string]interface{}{
		"B":  1234,
		"C":  12.34,
		"D":  "abcd",
		"T1": tm,
		"T2": timestamppb.New(tm),
		"U1": u,
		"U2": u.String(),
	}
	var s2 s2
	err := CopyV1(&s2, m)
	req.Nil(err)
	req.Equal(m["B"], s2.B)
	req.Equal(m["C"], s2.C)
	req.Equal(m["D"], string(s2.D))
	req.Equal(m["T1"], s2.T1.AsTime())
	req.Equal(m["T2"], timestamppb.New(s2.T2))
	req.Equal(m["U1"], u)
	req.Equal(m["U2"], s2.U2.String())
}

func TestPBMapToStructAdapters(t *testing.T) {
	req := require.New(t)
	tm := time.Unix(12345678, 0).UTC()
	u := uuid.New()
	var s2 s2
	s1, err := structpb.NewStruct(map[string]interface{}{
		"B":  1234,
		"C":  12.34,
		"D":  "abcd",
		"T1": tm.Format(time.RFC3339),
		"T2": tm.Format(time.RFC3339),
		"U1": u.String(),
		"U2": u.String(),
		"X1": []interface{}{"foo", "bar", "baz"},
	})
	req.Nil(err)
	err = CopyV1(&s2, s1)
	req.Nil(err)
	m := s1.AsMap()
	req.Equal(m["B"], float64(s2.B))
	req.Equal(m["C"], s2.C)
	req.Equal(m["D"], string(s2.D))
	req.Equal(m["T1"], s2.T1.AsTime().Format(time.RFC3339))
	req.Equal(m["T2"], s2.T2.Format(time.RFC3339))
	req.Equal(m["U1"], s2.U1)
	req.Equal(m["U2"], s2.U2.String())
	req.Equal(m["X1"].([]interface{})[1], s2.X1[1])
}

func TestStructToPBMapAdapters(t *testing.T) {
	req := require.New(t)
	tm := time.Unix(12345678, 0).UTC()
	u := uuid.New()
	s1 := s1{
		B:  1234,
		C:  12.34,
		D:  "abcd",
		T1: tm,
		T2: timestamppb.New(tm),
		U1: u,
		U2: u.String(),
		X1: []string{"foo", "bar", "baz"},
	}
	s2, err := structpb.NewStruct(nil)
	req.Nil(err)
	err = CopyV1(s2, &s1)
	req.Nil(err)
	m := s2.AsMap()
	req.Equal(float64(s1.B), m["B"])
	req.Equal(s1.C, m["C"])
	req.Equal(s1.D, m["D"])
	req.Equal(s1.T1.Format(time.RFC3339), m["T1"])
	req.Equal(s1.T2.AsTime().Format(time.RFC3339), m["T2"])
	req.Equal(s1.U1.String(), m["U1"])
	req.Equal(s1.U2, m["U2"])
	req.Equal(s1.X1[1], m["X1"].([]interface{})[1])
}

type stdUUIDAdapterSrc struct {
	U1 stduuid.UUID
	U2 string
}

type stdUUIDAdapterDst struct {
	U1 string
	U2 stduuid.UUID
}

func TestStdUUIDStructToPBMapAdapters(t *testing.T) {
	req := require.New(t)
	u := stduuid.NewV7()
	src := stdUUIDAdapterSrc{U1: u, U2: u.String()}
	dst, err := structpb.NewStruct(nil)
	req.Nil(err)
	err = CopyV1(dst, &src)
	req.Nil(err)
	m := dst.AsMap()
	req.Equal(u.String(), m["U1"])
	req.Equal(u.String(), m["U2"])
}

func TestStdUUIDPBMapToStructAdapters(t *testing.T) {
	req := require.New(t)
	u := stduuid.NewV7()
	src, err := structpb.NewStruct(map[string]interface{}{
		"U1": u.String(),
		"U2": u.String(),
	})
	req.Nil(err)
	var dst stdUUIDAdapterDst
	err = CopyV1(&dst, src)
	req.Nil(err)
	req.Equal(u.String(), dst.U1)
	req.Equal(u, dst.U2)
}

func TestStdUUIDMapToStructAdaptersEmptyString(t *testing.T) {
	req := require.New(t)
	m := map[string]interface{}{
		"U1": "",
		"U2": "",
	}
	var dst stdUUIDAdapterDst
	err := CopyV1(&dst, m)
	req.Nil(err)
	req.Equal(stduuid.Nil(), dst.U2)
}

func TestStdUUIDMapToStructAdaptersFailure(t *testing.T) {
	req := require.New(t)
	m := map[string]interface{}{
		"U1": "abcd",
		"U2": "not-a-uuid",
	}
	var dst stdUUIDAdapterDst
	req.Error(CopyV1(&dst, m))
}

type custSrc struct {
	X int
}

type custDst struct {
	Y string
}

type wrapperSrc struct {
	X *custSrc
}

type wrapperDst struct {
	X *custDst
}

func TestCustomConvCopy(t *testing.T) {
	req := require.New(t)
	var dst custDst
	conv := new(ConvertorOption)
	RegisterConv(conv, func(x *custSrc) (*custDst, error) {
		return &custDst{Y: strconv.Itoa(x.X)}, nil
	})
	err := CopyV1(&dst, &custSrc{X: 1234}, conv)
	req.Nil(err)
	req.Equal("1234", dst.Y)
}

func TestCustomConvInnerCopy(t *testing.T) {
	req := require.New(t)
	var dst wrapperDst
	conv := new(ConvertorOption)
	RegisterConv(conv, func(x *custSrc) (*custDst, error) {
		return &custDst{Y: strconv.Itoa(x.X)}, nil
	})
	err := CopyV1(&dst, &wrapperSrc{X: &custSrc{X: 1234}}, conv)
	req.Nil(err)
	req.Equal("1234", dst.X.Y)
}

type s3 struct {
	IDs []string
}

type s4 struct {
	IDs []uuid.UUID
}

func TestSliceCopy(t *testing.T) {
	req := require.New(t)
	u1, u2, u3 := uuid.New(), uuid.New(), uuid.New()
	src := &s3{IDs: []string{u1.String(), u2.String(), u3.String()}}
	var dst s4
	err := NewCopy(&dst, src)
	req.Nil(err)
	req.Equal(3, len(dst.IDs))
	req.Equal(u1, dst.IDs[0])
	req.Equal(u2, dst.IDs[1])
	req.Equal(u3, dst.IDs[2])
}

type iface1 interface {
	GetX() string
}

type impl1 struct {
	X string
}

type impl2 struct {
	X string
}

func (i *impl2) GetX() string { return i.X }

type s5 struct {
	Obj *impl1
}

type s6 struct {
	Obj iface1
}

func TestInterfaceCopy(t *testing.T) {
	req := require.New(t)
	src := &s5{Obj: &impl1{X: "abcd"}}
	var dst s6
	err := CopyV1(&dst, src, &FactoryOption{
		Builders: map[string]func() interface{}{
			"Obj": func() interface{} { return new(impl2) },
		},
	})
	req.Nil(err)
	req.Equal(src.Obj.X, dst.Obj.GetX())
}

var gr interface{}

func BenchmarkPBMapToStructAdapters(b *testing.B) {
	tm := time.Unix(12345678, 0).UTC()
	u := uuid.New()
	var r interface{}
	for i := 0; i < b.N; i++ {
		var s2 s2
		s1, err := structpb.NewStruct(map[string]interface{}{
			"B":  1234,
			"C":  12.34,
			"D":  "abcd",
			"T2": tm.Format(time.RFC3339),
			"U1": u.String(),
			"U2": u.String(),
		})
		if err != nil {
			b.Error(err)
		}
		if err := CopyV1(&s2, s1); err != nil {
			b.Error(err)
		}
		r = &s2
	}
	gr = r
}

func BenchmarkPBMapToStructJSON(b *testing.B) {
	tm := time.Unix(12345678, 0).UTC()
	u := uuid.New()
	var r interface{}
	for i := 0; i < b.N; i++ {
		var s2 s2
		s1, err := structpb.NewStruct(map[string]interface{}{
			"B":  1234,
			"C":  12.34,
			"D":  "abcd",
			"T2": tm.Format(time.RFC3339),
			"U1": u.String(),
			"U2": u.String(),
		})
		if err != nil {
			b.Error(err)
		}
		bs, err := json.Marshal(s1.AsMap())
		if err != nil {
			b.Error(err)
		}
		if err := json.Unmarshal(bs, &s2); err != nil {
			b.Error(err)
		}
		r = &s2
	}
	gr = r
}
