package control

import (
	"iter"
	"strconv"

	"github.com/goradd/serve/page"
)

const defaultPrefix = "c"

type cacher interface {
	generateId() string
	cacheControl(c ControlI)
	GetControl(id string) (c ControlI)
	SetGeneratedIdPrefix(prefix string)
}

// cache is a map of the live control objects that are associated with a form.
// It is a mixin for the form_base, and is a way to separate and encapsulate the cache
// aspect of a form for testing and maintenance.
type cache struct {
	reg     map[string]ControlI
	counter int
	prefix  string
}

func (r *cache) SetGeneratedIdPrefix(prefix string) {
	r.prefix = prefix
}

func (r *cache) generateId() string {
	if r.prefix == "" {
		r.prefix = defaultPrefix
	}
	var id string
	// append integer to prefix and confirm it is not already in the cache
	for {
		id = r.prefix
		// Convert string to byte slice
		buf := []byte(id)
		// Append integer directly to the buffer (base 10)
		r.counter++
		buf = strconv.AppendInt(buf, int64(r.counter), 10)
		// Convert back to string
		id = string(buf)
		if _, ok := r.reg[id]; !ok {
			break
		}
	}
	return id
}

func (r *cache) cacheControl(c ControlI) {
	if r.reg == nil {
		r.reg = make(map[string]ControlI)
	}
	if _, ok := r.reg[c.ID()]; ok {
		panic("caching duplicate control id: " + c.ID())
	}
	r.reg[c.ID()] = c
}

func (r *cache) GetControl(id string) (c ControlI) {
	c, _ = r.reg[id]
	return
}

func (r *cache) serialize(e page.Encoder) {
	if err := e.Encode(r.counter); err != nil {
		panic(err)
	}
	if err := e.Encode(r.prefix); err != nil {
		panic(err)
	}

	var l int = len(r.reg)
	if err := e.Encode(l); err != nil {
		panic(err)
	}

	for _, c := range r.reg {
		if err := e.Encode(registryId(c)); err != nil {
			panic(err)
		}
		c.Serialize(e)
	}
}

func (r *cache) deserialize(d page.Decoder) {
	if err := d.Decode(&r.counter); err != nil {
		panic(err)
	}
	if err := d.Decode(&r.prefix); err != nil {
		panic(err)
	}
	var l int
	if err := d.Decode(&l); err != nil {
		panic(err)
	}
	for i := 0; i < l; i++ {
		var registryID uint64
		if err := d.Decode(&registryID); err != nil {
			return
		}
		c := createRegisteredControl(registryID)
		c.Deserialize(d)
		r.reg[c.ID()] = c
	}
}

// AllControls returns an iterator that yields all the controls in the form,
// including the form itself, in no particular order.
func (r *cache) AllControls() iter.Seq[ControlI] {
	return func(yield func(ControlI) bool) {
		for _, child := range r.reg {
			if !yield(child) {
				return
			}
		}
	}
}
