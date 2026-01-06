package page

import (
	"encoding/gob"
	"io"
)

// NewEncoderFuncType is the signature for the NewPageEncoderFunc function
type NewEncoderFuncType func(w io.Writer) Encoder

// NewDecoderFuncType is the signature for the NewPageDecoderFunc function
type NewDecoderFuncType func(r io.Reader) Decoder

// NewEncoderFunc is a function that returns a new encoder that will be used to
// encode the page state which is put into the pagestate cache.
// All the page state pieces will use this encoder to encode its parts into the page state.
// By default, it uses a gob encoder to encode to binary.
// You might want to set your own encoder if your page state cache cannot store binary,
// in which case you should also set the NewPageDecoderFunc.
var NewEncoderFunc NewEncoderFuncType = func(w io.Writer) Encoder { return gob.NewEncoder(w) }

// NewDecoderFunc is a function that returns a new decoder that will be used to
// decode the page state which is put into the pagestate cache.
var NewDecoderFunc NewDecoderFuncType = func(r io.Reader) Decoder { return gob.NewDecoder(r) }

// Encoder defines objects that can encode.
type Encoder interface {
	Encode(v interface{}) error
}

// Decoder defines objects that can decode.
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
