// Package jpeg is a remix of the Golang JPEG decoder allowing access to and
// modification of the raw DCT coefficients. This is mostly taken from the
// Golang JPEG decoder but striped down to only support the JPEGs FFMPEG is
// producing.
// Somewhat Copyright (c) 2009 The Go Authors. All rights reserved.
package jpeg

import (
	"fmt"
	"io"
	"os"
)

// JPEG holds a JPEG file and implements thr Frame interface.
type JPEG struct {
	path string

	height int
	width  int
	ri     int

	comps [3]component
	huffs [2][4]*huffman
	bits  bits

	// The actual DCT coefficients we embed in.
	blocks    []block
	compIndex []int
	dirty     bool

	// Encoding related fields

	// quant is the scaled quantization tables, in zig-zag order.
	quant         [1][blockSize]byte
	eBits, eNBits uint32
}

// DecodeJPEG attempts to decode the given reader as JPEG data giving access to
// the raw DCT coefficients.
func DecodeJPEG(r io.Reader, path string) (*JPEG, error) {
	j := &JPEG{
		path: path,
	}

	return j, j.decode(r)
}

// Encode encodes the current JPEG data and writes it out as a jpeg file to the
// filePath provided when the JPEG was created (i.e. it overwrites the current
// jpeg on disk).
func (jp *JPEG) Encode() error {
	f, err := os.OpenFile(jp.path, os.O_RDWR, 0755)
	if err != nil {
		return fmt.Errorf("Could not open %q to write: %v", jp.path, err)
	}
	defer f.Close()

	return jp.encode(f)
}

// Size returns the total number of DCT coefficients in all blocks.
func (j *JPEG) Size() int {
	return len(j.blocks) * 64
}

// GetElement returns the ith DCT coefficient.
func (j *JPEG) GetElement(i int) int {
	if i < 0 {
		panic(fmt.Errorf("JPEG GetElement i < 0: %d", i))
	}
	if i >= j.Size() {
		panic(fmt.Errorf("JPEG GetElement i >= Size(). Size: %d, i: %d", j.Size(), i))
	}

	return (int)(j.blocks[i/64][i%64])

	panic(fmt.Errorf("Reached bottom of GetElement, this should never happen"))
}

// SetElement sets the ith DCT coefficient to val.
func (j *JPEG) SetElement(i, val int) {
	if i < 0 {
		panic(fmt.Errorf("JPEG GetElement i < 0: %d", i))
	}
	if i >= j.Size() {
		panic(fmt.Errorf("JPEG GetElement i >= Size(). Size: %d, i: %d", j.Size(), i))
	}

	j.blocks[i/64][i%64] = (int32)(val)
	j.dirty = true
}

// IsDirty returns true if the JPEG data has been modified.
func (j *JPEG) IsDirty() bool {
	return j.dirty
}
