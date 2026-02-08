package page

import (
	"fmt"
	"hash/fnv"
	"reflect"
)

type CreateFunc func() ControlI

// ControlRegistrySalt is used to generate unique ids in the control controlRegistry. However, if the control controlRegistry
// detects a collision, you will need to change this value and restart your app. If you have a running
// page controlCache, you should change the PageCacheVersion above as well to invalidate it.
var ControlRegistrySalt = "gs"

var controlRegistry = make(map[uint64]CreateFunc)
var controlRegistryIds = make(map[reflect.Type]uint64)

// RegisterControl registers the control for the serialize/deserialize process. Controls should call this
// for each control from an init() function. Pass in a function that will call new on your control
// type and that is all.
//
// Example:
//
//	init() {
//	  control.RegisterControl(func() page.ControlI {return new(MyControl)})
//	}
func RegisterControl(f CreateFunc) {
	// As a control is added to the controlRegistry, it is assigned an id. That id is used to identify a control
	// in the serialization and deserialization process.  We try to prevent the
	// addition of controls to an application from causing a change in these ids, since an id change will
	// also cause the current page controlCache to be invalidated. We use a hashing function, and a collision detector
	// to do that. If a collision is detected, it will panic, and you should change the hash salt and try again,
	// as well as bump the controlCache version to invalidate the controlCache.

	i := f()
	typ := reflect.TypeOf(i)
	for typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	if _, ok := controlRegistryIds[typ]; ok {
		panic("Registering duplicate control")
	}
	hash := fnv.New64()
	n := typ.Name()
	if n == "" {
		panic("type problem")
	}
	_, _ = hash.Write([]byte(ControlRegistrySalt))
	_, _ = hash.Write([]byte(typ.PkgPath()))
	_, _ = hash.Write([]byte(n))
	id := hash.Sum64()
	if f, ok := controlRegistry[id]; ok {
		typ2 := reflect.TypeOf(f())
		for typ2.Kind() == reflect.Ptr {
			typ2 = typ2.Elem()
		}

		panic("The control controlRegistry has detected a collision. " +
			typ2.Name() + " has collided with " + typ.Name() + ". " +
			"This is a very rare situation, but needs " +
			"to be fixed. To fix it, change the ControlRegistrySalt value, and also change the " +
			"PageCacheVersionID")
	}
	controlRegistry[id] = f
	controlRegistryIds[typ] = id
}

func registryId(i ControlI) uint64 {
	typ := i.TypeOf()
	id, ok := controlRegistryIds[typ]
	if !ok {
		panic("Control type is not registered: " + typ.String())
	}
	return id
}

// Create registered control returns a new control that has  Base initialized but nothing else.
func createRegisteredControl(registryID uint64) ControlI {
	var f CreateFunc
	var ok bool
	if f, ok = controlRegistry[registryID]; !ok {
		panic(fmt.Errorf("attempting to decode a control type that is not registered: %d", registryID))
	}
	c := f()
	c.initBase(c)
	return c
}

func controlIsRegistered(i ControlI) bool {
	typ := reflect.TypeOf(i)
	for typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	_, ok := controlRegistryIds[typ]
	return ok
}
