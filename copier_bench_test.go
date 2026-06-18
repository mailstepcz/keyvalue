package keyvalue

// Benchmarks quantifying the cost of routing pointer-bearing copies through a
// write-barriered reflect assignment instead of a raw byte copy (the GC-safety fix).
// Reuses the stressDomain/stressDTO/scalar* types from copier_stress_test.go.
//
//	cd go-common/keyvalue
//	GOTOOLCHAIN=go1.26.4 go test -bench=. -benchmem -run='^$' -benchtime=2s -count=6
//
// BenchmarkCopySameTypeScalar should be unchanged by the fix (pointer-free → still
// memcopy); the pointer benchmarks show the price of the barriered path.

import "testing"

func benchSrcDomain() *stressDomain { return newStressDomain(0x9e3779b97f4a7c15) }

// BenchmarkCopySameTypePointer isolates the fix's direct cost: identical struct types,
// every field on the same-type path; pointer fields now use the barriered reflect copy.
func BenchmarkCopySameTypePointer(b *testing.B) {
	cp, err := TypedCopierForPair[stressDomain, stressDomain]()
	if err != nil {
		b.Fatal(err)
	}
	src := benchSrcDomain()
	var dst stressDomain
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := cp(&dst, src); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCopySameTypeScalar is the control: pointer-free struct stays on the fast
// memcopy path, so it should be statistically identical before and after the fix.
func BenchmarkCopySameTypeScalar(b *testing.B) {
	cp, err := TypedCopierForPair[scalarDomain, scalarDomain]()
	if err != nil {
		b.Fatal(err)
	}
	src := scalarDomain{A: 1, B: 2, D: 3.5, E: 7}
	var dst scalarDomain
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := cp(&dst, &src); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCopyMixed is the realistic proto<->domain shape: uuid<->string, time<->ts,
// maybe<->ptr, []*nested deep copy plus same-type string/slice/pointer fields.
func BenchmarkCopyMixed(b *testing.B) {
	cp, err := TypedCopierForPair[stressDTO, stressDomain]()
	if err != nil {
		b.Fatal(err)
	}
	src := benchSrcDomain()
	var dst stressDTO
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := cp(&dst, src); err != nil {
			b.Fatal(err)
		}
	}
}
