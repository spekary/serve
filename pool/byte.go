package pool

import (
	"math/bits"
	"sync"
	"sync/atomic"

	"github.com/goradd/serve/log"
)

const sampleWindow = 1024 // How many samples to take before adjusting

// Pool is an adaptive buffer pool that will adjust its parameters over time based on the
// size of the buffers returned to it.
type Pool struct {
	start atomic.Int64
	max   atomic.Int64
	pool  sync.Pool

	mu        sync.Mutex
	sumLens   int64
	sampleCnt int
}

// New creates a new adaptive buffer pool.
//
// startSize is the initial size of buffers created.
//
// maxSize is the initial maximum size of a buffer that can be returned to the buffer pool.
// If a buffer bigger than maxSize is attempted in a return, it will be discarded.
// maxSize will grow if the average size of returned buffers is over 50% of the maxSize.
func New(startSize, maxSize int) *Pool {
	if startSize <= 0 {
		panic("startSize must be > 0")
	}
	if maxSize < startSize {
		panic("maxSize must be >= startSize")
	}

	p := &Pool{}
	p.start.Store(int64(startSize))
	p.max.Store(int64(maxSize))

	// Allocate using the *current* adaptive start size.
	p.pool.New = func() any {
		s := int(p.start.Load())
		return make([]byte, 0, s)
	}
	return p
}

// Get retrieves a byte slice from the pool.
func (p *Pool) Get() []byte {
	b := p.pool.Get().([]byte)
	return b[:0]
}

func (p *Pool) Put(b []byte) {
	if b == nil {
		return
	}

	l := int64(len(b)) // adapt based on length used
	c := int64(cap(b)) // pool/drop based on capacity retained
	if c <= 0 {
		return
	}

	start := p.start.Load()
	maxSize := p.max.Load()

	p.observeAndMaybeAdjust(l)

	// Pooling policy remains capacity-based.
	if c < start || c > maxSize {
		return
	}

	p.pool.Put(b[:0])
}

// observeAndMaybeAdjust accumulates sampleWindow LENGTH samples,
// then sets start to:
//
//	start = roundUpPow2( ceil(avgLen * 1.20) )
//
// If start reaches >=50% of max, it logs and doubles max.
func (p *Pool) observeAndMaybeAdjust(lengthBytes int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.sumLens += lengthBytes
	p.sampleCnt++

	if p.sampleCnt < sampleWindow {
		return
	}

	avg := p.sumLens / int64(sampleWindow)
	if avg < 1 {
		avg = 1
	}

	// target = ceil(avg * 1.20) = ceil(avg*6/5) = (avg*6 + 4)/5
	target := (avg*6 + 4) / 5
	if target < 2 {
		target = 2
	}

	target = roundUpPow2(target)

	curMax := p.max.Load()
	if target > curMax {
		target = curMax
	}

	p.start.Store(target)

	// If start approaches 50% of max, log and double max.
	if target*2 >= curMax {
		newMax := curMax * 2
		p.max.Store(newMax)
		log.Info(nil, "pool", "bytepool: start=%d reached >=50%% of max=%d; doubling max to %d", target, curMax, newMax)
	}

	// reset window
	p.sumLens = 0
	p.sampleCnt = 0
}

// Stats returns the current start and max values.
func (p *Pool) Stats() (start, max int) {
	return int(p.start.Load()), int(p.max.Load())
}

// roundUpPow2 returns the smallest power of two >= x, with x >= 1.
// For x in [1..2] it returns 2 to satisfy “multiple of 2”.
func roundUpPow2(n int64) int64 {
	if n <= 2 {
		return 2
	}
	if n > 1<<62 {
		return 1 << 62
	}
	return 1 << bits.Len64(uint64(n-1))
}
