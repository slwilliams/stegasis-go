// Package video provides video decode and encoding functionaltiy.
package video

import (
	"fmt"
)

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
	// WriteFrame overwrites the ith frame of the intermediate video with the
	// provided frame. Panics if i >= Frames() or i < 0. Note that this won't
	// actually trigger a write of the source video with the new frame data, you
	// should call Encode if you want this.
	WriteFrame(i int, f Frame)
	// Close closes the Codec.
	Close()
}

// Frame defines the interface for a Frame object.
type Frame interface {
	// Size returns the number of elements (i.e. which can be embedded in) in the
	// frame.
	Size() int
	// GetElement returns a pointer to the frames ith element. Panics if i >= Size()
	// or if i < 0.
	GetElement(i int) *int
	// SetDirty marks the frame as dirty. You must call this if you modify a frame
	// element to ensure the frame data is written back to disk.
	SetDirty()
	// IsDirty returns true iff the frame has been set dirty.
	IsDirty() bool
}

// motionJPEGCodec uses FFMPEG to decode ~any video into a sequence of JPEG
// images where we can embed data.
type motionJPEGCodec struct {
	filePath string
	frames   []Frame
}

// Decode converts the source video file to a sequence of JPEG images via
// FFMPEG and stores them in a tempory directory.
func (c *motionJPEGCodec) Decode() error {
	for _, f := range c.frames {
		if f.IsDirty() {
			// TODO: Write this frame out.
		}
	}
	return nil
}

// Encode converts the sequence of JPEG images to a motion JPEG video
// overwriting the original source video.
func (c *motionJPEGCodec) Encode() error {
	// TODO Implement me.
	return nil
}

// GetFrame returns the ith frame. Panics if i >= Frames() or i < 0.
func (c *motionJPEGCodec) GetFrame(i int) Frame {
	if i < 0 {
		panic(fmt.Errorf("GetFrame %d cannot be negative", i))
	}
	if i >= c.Frames() {
		panic(fmt.Errorf("GetFrame %d is larger than total frame count %d", i, c.Frames()))
	}
	return c.frames[i]
}

// Frames returns the number of frames within the video file.
func (c *motionJPEGCodec) Frames() int {
	return len(c.frames)
}

// WriteFrame overwrites the ith frame of the intermediate video with the
// provided frame. Panics if i >= Frames() or i < 0.
func (c *motionJPEGCodec) WriteFrame(i int, f Frame) {
	if i < 0 {
		panic(fmt.Errorf("GetFrame %d cannot be negative", i))
	}
	if i >= c.Frames() {
		panic(fmt.Errorf("GetFrame %d is larger than total frame count %d", i, c.Frames()))
	}
	c.frames[i] = f
}

// Close closes the motion JPEG codec.
func (c *motionJPEGCodec) Close() {
}

// NewMotionJPEGCodec returns a new motion JPEG codec.
func NewMotionJPEGCodec(path string) (Codec, error) {
	return &motionJPEGCodec{
		filePath: path,
	}, nil
}
