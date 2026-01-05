// Package pool implements a buffer pool that is used by the framework and can be used by
// your application.
//
// See Pool for particulars of the features of the default pool.
//
// Call pool.Get() to return a buffer from the common buffer pool, which is suitable for small, quick
// buffering operations that you use and dispose of. To be safe, you should call
// defer func() {pool.Put()}() immediately after getting a buffer.
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
// Also, only put back buffers that you got earlier.
// A good practice is to defer a call to Put immediately after you call Get.
// If the buffer contains sensitive information, you should call clear(buf) before
// returning it to the buffer pool.
func Put(b []byte) {
	BytePool.Put(b)
}

// GetBuffer returns a byte buffer from the buffer pool.
// Call PutBuffer after you are done with the buffer.
func GetBuffer() *bytes.Buffer {
	b := BytePool.Get()
	return bytes.NewBuffer(b)
}

// PutBuffer return the buffer to the buffer pool.
// Typically, you should do this after a defer.
// Example:
//
//	defer PutBuffer(buf)
//
// Note: As opposed to Put, you do not need to wrap PutBuffer in a func wrapper
// because the buf owns the underlying memory and keeps track of reallocations.
func PutBuffer(buf *bytes.Buffer) {
	BytePool.Put(buf.Bytes())
}

func init() {
	// Create a default buffer pool for common tasks like
	// the pagecache or temporary drawing buffers.
	BytePool = New(BytePoolStartSize, BytePoolMaxSize)
}
