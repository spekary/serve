package pool

import (
	"runtime"
	"sync"
	"testing"
	"unsafe"
)

// Prevent compiler from optimizing away work.
var sinkBytes []byte
var sinkInt int

const (
	writeN = 6000
)

func BenchmarkPool(b *testing.B) {
	p := New(4<<10, 64<<10)

	payload := make([]byte, writeN)
	for i := range payload {
		payload[i] = byte(i)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		raw := p.Get()
		raw = append(raw, payload...)

		sinkInt += len(raw)
		sinkBytes = raw
		p.Put(raw)
	}
	start, m := p.Stats()
	b.Logf("start: %d, max: %d", start, m)
}

func benchOnce(payload []byte) {
	raw := Get()
	defer func() { Put(raw) }() // captures raw by reference

	raw = append(raw, payload...)
	sinkInt += len(raw)
	sinkBytes = raw
}

func BenchmarkGlobalPoolWithDefer(b *testing.B) {
	//p := New(4<<10, 64<<10)

	payload := make([]byte, writeN)
	for i := range payload {
		payload[i] = byte(i)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		benchOnce(payload)
	}
}

func benchOnceBuffer(payload []byte) {
	buf := GetBuffer()
	defer PutBuffer(buf)

	buf.Write(payload)
	sinkInt += len(buf.Bytes())
	sinkBytes = buf.Bytes()
}

func BenchmarkGlobalBufferPoolWithDefer(b *testing.B) {
	//p := New(4<<10, 64<<10)

	payload := make([]byte, writeN)
	for i := range payload {
		payload[i] = byte(i)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		benchOnceBuffer(payload)
	}
}

func Benchmark2Allocs(b *testing.B) {
	payload := make([]byte, writeN)
	for i := range payload {
		payload[i] = byte(i)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf := make([]byte, 0, 200)
		buf = append(buf, payload...)

		sinkInt += len(buf)
		sinkBytes = buf
	}
}

func Benchmark1Alloc(b *testing.B) {
	payload := make([]byte, writeN)
	for i := range payload {
		payload[i] = byte(i)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var buf []byte
		buf = append(buf, payload...)

		sinkInt += len(buf)
		sinkBytes = buf
	}
}

func TestNewPanicsOnInvalidArgs(t *testing.T) {
	t.Run("startSize<=0 panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("expected panic for startSize<=0, got none")
			}
		}()
		_ = New(0, 1)
	})

	t.Run("maxSize<startSize panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("expected panic for maxSize<startSize, got none")
			}
		}()
		_ = New(10, 9)
	})
}

func TestNewInitialStats(t *testing.T) {
	p := New(4<<10, 64<<10)
	start, max := p.Stats()
	if start != 4<<10 {
		t.Fatalf("start: got %d want %d", start, 4<<10)
	}
	if max != 64<<10 {
		t.Fatalf("max: got %d want %d", max, 64<<10)
	}
}

func TestRoundUpPow2(t *testing.T) {
	tests := []struct {
		in   int64
		want int64
	}{
		{0, 2},
		{1, 2},
		{2, 2},
		{3, 4},
		{4, 4},
		{5, 8},
		{7, 8},
		{8, 8},
		{9, 16},
		{15, 16},
		{16, 16},
		{17, 32},
		{63, 64},
		{64, 64},
		{65, 128},
	}
	for _, tt := range tests {
		got := roundUpPow2(tt.in)
		if got != tt.want {
			t.Fatalf("roundUpPow2(%d): got %d want %d", tt.in, got, tt.want)
		}
	}
}

func TestObserveAdjustsStartToPow2AboveAvgPlus20Percent(t *testing.T) {
	// Choose max large enough to avoid clamping.
	p := New(4, 4096)

	// Use a constant "lengthBytes" so avg is exact.
	// avg = 100
	// target = ceil(100*1.2) = 120
	// roundUpPow2(120) = 128
	const lengthBytes = int64(100)
	for i := 0; i < sampleWindow; i++ {
		p.observeAndMaybeAdjust(lengthBytes)
	}

	start, max := p.Stats()
	if max != 4096 {
		t.Fatalf("max changed unexpectedly: got %d want %d", max, 4096)
	}
	if start != 128 {
		t.Fatalf("start after adjust: got %d want %d", start, 128)
	}

	// Ensure the window reset and does not immediately adjust again.
	// Add fewer than sampleWindow samples; start should remain the same.
	for i := 0; i < sampleWindow-1; i++ {
		p.observeAndMaybeAdjust(lengthBytes)
	}
	start2, _ := p.Stats()
	if start2 != start {
		t.Fatalf("start changed before full window: got %d want %d", start2, start)
	}
}

func TestObserveClampsStartToMax(t *testing.T) {
	// Set max small enough that the computed target would exceed it.
	p := New(4, 256)

	// avg = 200
	// target = ceil(240) = 240
	// roundUpPow2(240) = 256 (== max) => clamped to 256
	const lengthBytes = int64(200)
	for i := 0; i < sampleWindow; i++ {
		p.observeAndMaybeAdjust(lengthBytes)
	}

	start, max := p.Stats()
	if start != 256 {
		t.Fatalf("start clamp: got %d want %d", start, 256)
	}
	if max != 512 {
		// NOTE: because start==max, it triggers doubling per your rule: start*2 >= max.
		t.Fatalf("max should have doubled: got %d want %d", max, 512)
	}
}

func TestObserveDoublesMaxWhenStartReachesHalfMax(t *testing.T) {
	p := New(4, 256)

	// Pick a length that yields a target that rounds up to 256 (then clamped),
	// so start becomes 256 and triggers doubling (start*2 >= max).
	// avg = 110
	// target = ceil(132) = 132
	// roundUpPow2(132) = 256 -> clamp to 256 -> doubles max to 512
	const lengthBytes = int64(110)
	for i := 0; i < sampleWindow; i++ {
		p.observeAndMaybeAdjust(lengthBytes)
	}

	start, max := p.Stats()
	if start != 256 {
		t.Fatalf("start: got %d want %d", start, 256)
	}
	if max != 512 {
		t.Fatalf("max: got %d want %d", max, 512)
	}
}

func TestPutHandlesNilAndZeroCapSafely(t *testing.T) {
	p := New(8, 64)

	// nil should be ignored
	p.Put(nil)

	// cap==0 slice should be ignored
	var zero []byte
	p.Put(zero)

	// sanity: Get should still work
	b := p.Get()
	if len(b) != 0 {
		t.Fatalf("Get: expected len 0, got %d", len(b))
	}
	if cap(b) < 8 {
		t.Fatalf("Get: expected cap >= 8, got %d", cap(b))
	}
}

// ptr1 returns a stable pointer to the first byte of the backing array.
// It requires cap(b) > 0.
func ptr1(b []byte) unsafe.Pointer {
	return unsafe.Pointer(&b[:1][0])
}

func TestPut_EarlyReturnsDontAffectCounters(t *testing.T) {
	p := New(1024, 4096)

	// nil -> return early, should not change sampleCnt/sumLens
	p.Put(nil)

	// cap==0 slice -> return early, should not change sampleCnt/sumLens
	var z []byte
	p.Put(z)

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.sampleCnt != 0 {
		t.Fatalf("sampleCnt changed on early returns: got %d want 0", p.sampleCnt)
	}
	if p.sumLens != 0 {
		t.Fatalf("sumLens changed on early returns: got %d want 0", p.sumLens)
	}
}

func TestPut_ExercisesDropBranches(t *testing.T) {
	p := New(1024, 4096)

	// cap < start -> should be dropped (but observeAndMaybeAdjust still runs)
	tooSmall := make([]byte, 10, 1023)
	p.Put(tooSmall)

	// cap > max -> should be dropped (but observeAndMaybeAdjust still runs)
	tooLarge := make([]byte, 10, 4097)
	p.Put(tooLarge)

	p.mu.Lock()
	defer p.mu.Unlock()

	// Both calls had cap > 0, so both should have contributed to adaptation samples.
	if p.sampleCnt != 2 {
		t.Fatalf("expected sampleCnt 2, got %d", p.sampleCnt)
	}
	if p.sumLens != 20 {
		t.Fatalf("expected sumLens 20, got %d", p.sumLens)
	}
}

func TestPut_ExercisesAcceptBranch_NoPanic(t *testing.T) {
	p := New(1024, 4096)

	// cap in [start,max] should go through the "accept path" (i.e. not return early)
	ok := make([]byte, 50, 2048)
	p.Put(ok)

	// We cannot deterministically assert it is retrievable from sync.Pool, but we can
	// assert the observation counters advanced.
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.sampleCnt != 1 {
		t.Fatalf("expected sampleCnt 1, got %d", p.sampleCnt)
	}
	if p.sumLens != 50 {
		t.Fatalf("expected sumLens 50, got %d", p.sumLens)
	}
}

func TestPut_TriggersAdjustmentAndStartMath(t *testing.T) {
	// Set max large enough to avoid clamping/doubling in this test.
	p := New(4, 4096)

	// Feed exactly sampleWindow samples with len=100, cap within bounds.
	// avg = 100
	// target = ceil(100*1.2) = 120
	// roundUpPow2(120) = 128
	for i := 0; i < sampleWindow; i++ {
		b := make([]byte, 100, 256)
		p.Put(b)
	}

	start, max := p.Stats()
	if max != 4096 {
		t.Fatalf("max changed unexpectedly: got %d want %d", max, 4096)
	}
	if start != 128 {
		t.Fatalf("start did not adapt as expected: got %d want %d", start, 128)
	}

	// Window should have reset after adjustment.
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.sampleCnt != 0 || p.sumLens != 0 {
		t.Fatalf("expected window reset; got sampleCnt=%d sumLens=%d", p.sampleCnt, p.sumLens)
	}
}

func TestPut_DoublesMaxWhenStartReachesHalfMax(t *testing.T) {
	p := New(4, 256)

	// len=110 => avg=110 => target=ceil(132)=132 => roundUpPow2=256 => clamp to 256
	// then target*2 >= curMax => doubles max to 512
	for i := 0; i < sampleWindow; i++ {
		b := make([]byte, 110, 256)
		p.Put(b)
	}

	start, max := p.Stats()
	if start != 256 {
		t.Fatalf("start: got %d want %d", start, 256)
	}
	if max != 512 {
		t.Fatalf("max: got %d want %d", max, 512)
	}
}

func TestPool_ConcurrentGetPut_NoPanic(t *testing.T) {
	// Run with:
	//   go test -race
	//
	// This test is about safety (no races/panics/deadlocks), not deterministic reuse.
	p := New(4<<10, 64<<10)

	const (
		goroutines = 32
		iters      = 5000
		payloadLen = 200
	)

	payload := make([]byte, payloadLen)
	for i := range payload {
		payload[i] = byte(i)
	}

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				b := p.Get()
				b = append(b, payload...)
				p.Put(b)

				if i%256 == 0 {
					runtime.Gosched()
				}
			}
		}()
	}

	wg.Wait()

	start, max := p.Stats()
	if start <= 0 || max <= 0 {
		t.Fatalf("invalid stats: start=%d max=%d", start, max)
	}
	if max < start {
		t.Fatalf("invalid stats ordering: start=%d max=%d", start, max)
	}
}
