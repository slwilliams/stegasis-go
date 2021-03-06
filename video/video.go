// Package video provides video decode and encoding functionaltiy.
package video

// Codec defines the interface for a video codec. That is, providing access to
// individual frames so we can steganographically embed data within them.
type Codec interface {
	// Decode decodes the source video file into an intermediate format to allow
	// us direct access to the frame data. This must be called first.
	Decode() error
	// Encode writes back any modified frames into the source video file. This
	// can be called multiple times during the lifetime of the Codec.
	Encode() error
	// GetFrame returns the ith frame. Panics if i >= Frames() or i < 0.
	GetFrame(i int) Frame
	// Close closes the Codec.
	Close()
}

// Frame defines the interface for a Frame object.
type Frame interface {
	// Size returns the number of elements (which can be embedded in) in the
	// frame.
	Size() int
	// GetElement returns a pointer to the frames ith element. Panics if i >= Size()
	// or if i < 0.
	GetElement(i int) int
	// SetElement sets the ith element to val. Panics if i >= Size() or if i < 0.
	// Will cause the frame to be considered dirty.
	SetElement(i, val int)
	// IsDirty returns true iff the frame is considered dirty.
	IsDirty() bool
}
