// Package pool implements a buffer pool that is used by the framework and can be used by
// your application.
//
// See Pool for particulars of the features of the default pool.
//
// Call pool.Get() to return a buffer from the common buffer pool, which is suitable for small, quick
// buffering operations that you use and dispose of.
// Call pool.Put(b) to return a buffer to a pool.
// You can use defer to make sure a buffer ets returned to the pool, but it is not required,
// since a buffer that is not returned is eventually just cleaned up.
//
// If you do call pool.Put with a defer, be sure to wrap it in a function wrapper.
//
// Example:
//
//	{
//	  b := pool.Get()
//	  defer func() {pool.Put(b)} // the wrapper is important here in case b is reallocated via append.
//	  // do buffer work here
//	  b = append(b, mybytes...)
//	}
//
// The buffer pool has some protection against running out of memory, but not an absolute protection.
// The preferred way to protect against memory exhaustion is to actively monitor memory use in the app,
// and when memory is getting low, behave differently, like return a 503 error stating the server is too
// busy to respond to the request.
package pool

import "bytes"

// BytePool is the common buffer pool for small buffers.
// It is used by the framework for binary compressed data for the page cache
// and for small draw buffering.
// Call pool.New to create a specialized buffer pool if this will not suffice.
var BytePool BytePoolI

type BytePoolI interface {
	Get() []byte
	Put(b []byte)
}

// BytePoolStartSize is the default starting size for the buffer pool
var BytePoolStartSize = 2 << 10 // 2KB
// BytePoolMaxSize is the default max size for the buffer pool
var BytePoolMaxSize = 10 << 10 // 10KB, any bigger the buffer will be unallocated

// Get returns an empty buffer from the common pool.
func Get() []byte {
	return BytePool.Get()
}

// Put puts a buffer back into the common pool.
//
// Be very careful that you do not refer to the buffer after putting it back,
// including using a slice of a buffer.
//
// Also, only put back buffers that you got earlier.
//
// You can use defer to make sure a buffer its returned to the pool, but it is not required,
// since a buffer that is not returned is eventually just cleaned up.
//
// If you do call pool.Put with a defer, be sure to wrap it in a function wrapper.
//
// Example:
//
//	{
//	  b := pool.Get()
//	  defer func() {pool.Put(b)} // the wrapper is important here in case b is reallocated via append.
//	  // do buffer work here
//	  b = append(b, mybytes...)
//	}
func Put(b []byte) {
	BytePool.Put(b)
}

// GetBuffer returns a byte buffer from the buffer pool.
// Call PutBuffer after you are done with the buffer.
func GetBuffer() *bytes.Buffer {
	b := BytePool.Get()
	return bytes.NewBuffer(b)
}

// PutBuffer returns the buffer to the buffer pool.
func PutBuffer(buf *bytes.Buffer) {
	BytePool.Put(buf.Bytes())
}

func init() {
	// Create a default buffer pool for common tasks like
	// the pagecache or temporary drawing buffers.
	BytePool = New(BytePoolStartSize, BytePoolMaxSize)
}
