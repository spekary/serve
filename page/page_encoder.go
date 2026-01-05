package page

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"io"
)

// These objects and functions are helpers in the page serialization process. Serialization is a big nut to crack in Go,
// with many opinions and options on how to implement. This current implementation tries to be flexible and supportable first, before fast.
// As goradd matures, this can evolve into something that is more optimized. It is essentially implemented as a service that is
// initialized at startup time.
// This is only needed when we are serializing pages. Not needed on single machine implementations that keeps the pagecache in memory.

var pageEncoder PageEncoderI

type PageEncoderI interface {
	NewEncoder(b *bytes.Buffer) Encoder
	NewDecoder(b *bytes.Buffer) Decoder
}

func SetPageEncoder(e PageEncoderI) {
	pageEncoder = e
}

// Encoder defines objects that can be encoded into a pagestate.
type Encoder interface {
	Encode(v interface{}) error
}

// Decoder defines objects that can be decoded from a pagestate. If the object does not implement this, we will look for GobDecode support.
type Decoder interface {
	Decode(v interface{}) error
}

// Serializable defines the interface that allows an object to be encodable using a pre-set encoder. This saves time
// on memory allocations/deallocations, which might be extensive.
// Controls are Serializable by default. Other objects that contain controls, or that are not gob.Encoders should implement
// this as well if they are part of the pagestate.
type Serializable interface {
	Serialize(e Encoder)
	Deserialize(d Decoder)
}

type GobPageEncoder struct {
}

type GobSerializer struct {
	*gob.Encoder
	//*json.Encoder
}

type GobDeserializer struct {
	*gob.Decoder
	//*json.Decoder
}

func (e GobPageEncoder) NewEncoder(b *bytes.Buffer) Encoder {
	return &GobSerializer{gob.NewEncoder(b)}
}

func (e GobPageEncoder) NewDecoder(b *bytes.Buffer) Decoder {
	return &GobDeserializer{gob.NewDecoder(b)}
}

func (e GobSerializer) Encode(v interface{}) (err error) {
	return e.Encoder.Encode(v)
}

func (e GobDeserializer) Decode(v interface{}) (err error) {
	return e.Decoder.Decode(v)
}

// EncodeString efficiently encodes a string into the byte slice b, returning a
// possible new byte slice.
//
// Call DecodeString to decode the string from the buffer.
func EncodeString(b []byte, s string) []byte {
	b = binary.LittleEndian.AppendUint32(b, uint32(len(s)))
	b = append(b, s...)
	return b
}

// DecodeString decodes a string that was encoded using EncodeString,
// returning the string, the number of bytes consumed from the byte slice b,
// and any errors.
func DecodeString(b []byte) (s string, n int, err error) {
	if len(b) < 4 {
		return "", n, io.ErrUnexpectedEOF
	}
	l := int(binary.LittleEndian.Uint32(b))
	n += 4

	if len(b) < n+l {
		return "", 0, io.ErrUnexpectedEOF
	}
	s = string(b[n : n+l])
	n += l
	return
}

// EncodeStringSlice will encode the given slice of strings into byte slice b,
// returning a possible new byts slice.
func EncodeStringSlice(b []byte, ss []string) []byte {
	b = binary.LittleEndian.AppendUint32(b, uint32(len(ss)))
	for _, s := range ss {
		b = EncodeString(b, s)
	}
	return b
}

// DecodeStringSlice will return a slice of strings that was encoded using EncodeStringSlice.
// It also returns the number of bytes consumed in b, and any errors.
func DecodeStringSlice(b []byte) (ss []string, n int, err error) {
	if len(b) < 4 {
		return nil, 0, io.ErrUnexpectedEOF
	}
	count := int(binary.LittleEndian.Uint32(b))
	n += 4
	var n2 int
	for i := 0; i < count; i++ {
		var s string
		if s, n2, err = DecodeString(b[n:]); err != nil {
			return
		}
		n += n2
		ss = append(ss, s)
	}
	return
}
