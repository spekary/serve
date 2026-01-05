package control

import (
	"fmt"
	"hash/fnv"
	"reflect"
)

type CreateFunc func() ControlI

// RegistrySalt is used to generate unique ids in the control registry. However, if the control registry
// detects a collision, you will need to change this value and restart your app. If you have a running
// page cache, you should change the PageCacheVersion above as well to invalidate it.
var RegistrySalt = "gs"

var registry = make(map[uint64]CreateFunc)
var registryIds = make(map[reflect.Type]uint64)

// Register registers the control for the serialize/deserialize process. You should call this
// for each control from an init() function. Pass in a function that will call new on your control
// type and that is all.
//
// Example:
//
//	init() {
//	  control.Register(func() control.ControlI {return new(MyControl)})
//	}
func Register(f CreateFunc) {
	// As a control is added to the registry, it is assigned an id. That id is used to identify a control
	// in the serialization and deserialization process.  We try to prevent the
	// addition of controls to an application from causing a change in these ids, since an id change will
	// also cause the current page cache to be invalidated. We use a hashing function, and a collision detector
	// to do that. If a collision is detected, it will panic, and you should change the hash salt and try again,
	// as well as bump the cache version to invalidate the cache.

	i := f()
	typ := reflect.TypeOf(i)
	for typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	if _, ok := registryIds[typ]; ok {
		panic("Registering duplicate control")
	}
	hash := fnv.New64()
	n := typ.Name()
	if n == "" {
		panic("type problem")
	}
	_, _ = hash.Write([]byte(RegistrySalt))
	_, _ = hash.Write([]byte(typ.PkgPath()))
	_, _ = hash.Write([]byte(n))
	id := hash.Sum64()
	if f, ok := registry[id]; ok {
		typ2 := reflect.TypeOf(f())
		for typ2.Kind() == reflect.Ptr {
			typ2 = typ2.Elem()
		}

		panic("The control registry has detected a collision. " +
			typ2.Name() + " has collided with " + typ.Name() + ". " +
			"This is a very rare situation, but needs " +
			"to be fixed. To fix it, change the RegistrySalt value, and also change the " +
			"PageCacheVersionID")
	}
	registry[id] = f
	registryIds[typ] = id
}

func registryId(i ControlI) uint64 {
	typ := i.TypeOf()
	id, ok := registryIds[typ]
	if !ok {
		panic("ControlBase type is not registered: " + typ.String())
	}
	return id
}

// Create registered control returns a new control that has  Base initialized but nothing else.
func createRegisteredControl(registryID uint64) ControlI {
	var f CreateFunc
	var ok bool
	if f, ok = registry[registryID]; !ok {
		panic(fmt.Errorf("attempting to decode a control type that is not registered: %d", registryID))
	}
	c := f()
	c.initBase(c)
	return c
}

func controlIsRegistered(i ControlI) bool {
	typ := i.TypeOf()
	_, ok := registryIds[typ]
	return ok
}
