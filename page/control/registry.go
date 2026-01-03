package control

import (
	"strconv"
)

const defaultPrefix = "c"

type register interface {
	generateId() string
	registerControl(c ControlI)
	RegisteredControl(id string) (c ControlI)
}

// registry is a map of the controls that are associated with a
type registry struct {
	reg     map[string]ControlI
	counter int
	prefix  string
}

func (r *registry) SetGeneratedIdPrefix(prefix string) {
	r.prefix = prefix
}
func (r *registry) generateId() string {
	if r.prefix == "" {
		r.prefix = defaultPrefix
	}
	var id string
	// append integer to prefix and confirm it is not already in the registry
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

func (r *registry) registerControl(c ControlI) {
	if r.reg == nil {
		r.reg = make(map[string]ControlI)
	}
	r.reg[c.ID()] = c
}

func (r *registry) RegisteredControl(id string) (c ControlI) {
	c, _ = r.reg[id]
	return
}
