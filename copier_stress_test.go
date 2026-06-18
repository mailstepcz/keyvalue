package keyvalue

// Diagnostic stress harness for the company-wide-prod heap-corruption investigation
// (keyvalue unsafe copiers as the leading suspect). Not part of normal CI: it only
// runs when KV_STRESS_SECONDS is set, e.g.
//
//	cd go-common/keyvalue
//	KV_STRESS_SECONDS=20 GOMEMLIMIT=48MiB GOGC=10 \
//	  GODEBUG=clobberfree=1,gccheckmark=1 \
//	  GOTOOLCHAIN=local go test -race -run TestCopierStress -v
//
// Goal: provoke "found pointer to free object" / "found bad pointer in Go heap", or a
// -race / checkptr report inside copier.go / transmute.go, under prod-like GC pressure.

import (
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mailstepcz/maybe"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type stressNested struct {
	ID   uuid.UUID
	Name string
}

type stressNestedDTO struct {
	ID   string
	Name string
}

// stressDomain mirrors a typical domain entity: pointers, slices, slice-of-pointers,
// maybe, uuid and time fields.
type stressDomain struct {
	ID       uuid.UUID
	Name     string
	Note     *string
	Tags     []string
	Children []*stressNested
	Opt      maybe.Maybe[string]
	Child    *stressNested
	Count    int
	When     time.Time
}

// stressDTO mirrors the matching proto/DTO side, forcing the converting copier paths
// (uuid<->string, time<->timestamp, maybe<->ptr, []*nested deep copy).
type stressDTO struct {
	ID       string
	Name     string
	Note     *string
	Tags     []string
	Children []*stressNestedDTO
	Opt      *string
	Child    *stressNested
	Count    int
	When     *timestamppb.Timestamp
}

// sink keeps copied-out data observable so the compiler can't elide the work, and so
// some pointers stay live across GC cycles while most churn.
var stressSink atomic.Pointer[stressDTO]

func TestCopierStress(t *testing.T) {
	secs := os.Getenv("KV_STRESS_SECONDS")
	if secs == "" {
		t.Skip("set KV_STRESS_SECONDS to run the heap-corruption stress harness")
	}
	dur, err := strconv.Atoi(secs)
	if err != nil || dur <= 0 {
		t.Fatalf("bad KV_STRESS_SECONDS=%q", secs)
	}

	toDTO, err := TypedCopierForPair[stressDTO, stressDomain]()
	if err != nil {
		t.Fatalf("building domain->dto copier: %v", err)
	}
	toDomain, err := TypedCopierForPair[stressDomain, stressDTO]()
	if err != nil {
		t.Fatalf("building dto->domain copier: %v", err)
	}
	sliceToDTO, err := SliceCopierForPair[stressDTO, stressDomain]()
	if err != nil {
		t.Fatalf("building slice copier: %v", err)
	}

	deadline := time.Now().Add(time.Duration(dur) * time.Second)
	workers := runtime.GOMAXPROCS(0) * 3
	if workers < 4 {
		workers = 4
	}
	t.Logf("stress: workers=%d duration=%ds GOMAXPROCS=%d", workers, dur, runtime.GOMAXPROCS(0))

	var ops atomic.Uint64
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			r := uint64(seed*2654435761 + 1)
			next := func() uint64 { r ^= r << 13; r ^= r >> 7; r ^= r << 17; return r }
			for time.Now().Before(deadline) {
				n := int(next()%6) + 1
				batch := make([]*stressDomain, n)
				for i := range batch {
					batch[i] = newStressDomain(next())
				}

				// converting copy domain -> dto (uuid->string, time->ts, maybe->ptr, []*nested deep)
				dtos, err := sliceToDTO(batch)
				if err != nil {
					t.Errorf("slice copy: %v", err)
					return
				}

				// round-trip each back dto -> domain and re-forward, hammering the
				// allocation + pointer-write paths.
				for _, d := range dtos {
					var back stressDomain
					if err := toDomain(&back, d); err != nil {
						t.Errorf("dto->domain: %v", err)
						return
					}
					var fwd stressDTO
					if err := toDTO(&fwd, &back); err != nil {
						t.Errorf("domain->dto: %v", err)
						return
					}
					touchDTO(&fwd)
					if next()%64 == 0 {
						stressSink.Store(&fwd)
					}
				}

				// bystander garbage of the elemsizes seen in prod dumps (16/32/896)
				// for the GC to scan/sweep alongside the copied objects.
				for _, sz := range [...]int{16, 32, 896} {
					b := make([]byte, sz)
					b[0] = byte(next())
					_ = b
				}

				if ops.Add(1)%4096 == 0 {
					runtime.GC()
				}
			}
		}(w)
	}
	wg.Wait()
	t.Logf("stress: completed ops=%d (no corruption detected)", ops.Load())
}

// scalarDomain / scalarDTO are the pointer-free control: identical layout, no pointers,
// so the copier's raw byte writes carry no pointer words and can't miss a GC write barrier.
type scalarDomain struct {
	A int
	B int64
	C [16]byte
	D float64
	E uint32
}

type scalarDTO struct {
	A int
	B int64
	C [16]byte
	D float64
	E uint32
}

// TestCopierStressScalar is the control: same stress, pointer-free types. Expected to
// survive — if it does while TestCopierStress crashes, the trigger is keyvalue copying
// pointers without a write barrier, not the harness or the runtime.
func TestCopierStressScalar(t *testing.T) {
	secs := os.Getenv("KV_STRESS_SECONDS")
	if secs == "" {
		t.Skip("set KV_STRESS_SECONDS to run the heap-corruption stress harness")
	}
	dur, _ := strconv.Atoi(secs)
	if dur <= 0 {
		dur = 10
	}
	cp, err := TypedCopierForPair[scalarDTO, scalarDomain]()
	if err != nil {
		t.Fatalf("building scalar copier: %v", err)
	}
	deadline := time.Now().Add(time.Duration(dur) * time.Second)
	workers := runtime.GOMAXPROCS(0) * 3
	var ops atomic.Uint64
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			r := uint64(seed*2654435761 + 1)
			next := func() uint64 { r ^= r << 13; r ^= r >> 7; r ^= r << 17; return r }
			for time.Now().Before(deadline) {
				src := scalarDomain{A: int(next()), B: int64(next()), D: float64(next()), E: uint32(next())}
				var dst scalarDTO
				if err := cp(&dst, &src); err != nil {
					t.Errorf("scalar copy: %v", err)
					return
				}
				_ = dst.A + int(dst.E)
				for _, sz := range [...]int{16, 32, 896} {
					b := make([]byte, sz)
					b[0] = byte(next())
					_ = b
				}
				if ops.Add(1)%4096 == 0 {
					runtime.GC()
				}
			}
		}(w)
	}
	wg.Wait()
	t.Logf("scalar control: completed ops=%d (no corruption — expected)", ops.Load())
}

func newStressDomain(seed uint64) *stressDomain {
	note := "note-" + strconv.FormatUint(seed, 36)
	d := &stressDomain{
		ID:    uuid.New(),
		Name:  "name-" + strconv.FormatUint(seed, 16),
		Note:  &note,
		Tags:  []string{"a" + strconv.FormatUint(seed%97, 10), "b", "c"},
		Child: &stressNested{ID: uuid.New(), Name: "child"},
		Count: int(seed % 1000),
		When:  time.Unix(int64(seed%1_000_000), 0),
	}
	nc := int(seed%5) + 1
	d.Children = make([]*stressNested, nc)
	for i := range d.Children {
		d.Children[i] = &stressNested{ID: uuid.New(), Name: "n" + strconv.Itoa(i)}
	}
	if seed%2 == 0 {
		d.Opt = maybe.Unit("opt-" + strconv.FormatUint(seed%31, 10))
	} else {
		d.Opt = maybe.Nothing[string]()
	}
	return d
}

// touchDTO reads every pointer-bearing field so the values are materialised (and any
// bad pointer is dereferenced) rather than optimised away.
func touchDTO(d *stressDTO) {
	_ = len(d.ID) + len(d.Name) + d.Count
	if d.Note != nil {
		_ = len(*d.Note)
	}
	for _, s := range d.Tags {
		_ = len(s)
	}
	for _, c := range d.Children {
		if c != nil {
			_ = len(c.ID) + len(c.Name)
		}
	}
	if d.Child != nil {
		_ = len(d.Child.Name)
	}
	if d.Opt != nil {
		_ = len(*d.Opt)
	}
	if d.When != nil {
		_ = d.When.AsTime().Unix()
	}
}
