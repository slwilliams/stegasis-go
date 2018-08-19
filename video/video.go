// Package video provides video decode and encoding functionaltiy.
package video

import ()

// Codec defines the interface for a video codec. That is, providing access to
// individual frames so we can steganographically embed data within them.
type Codec interface {
	// Decode decodes the source video file into an intermediate format to allow
	// us direct access to the frame data. This must be called first.
	Decode() error
	// Encode writes back all of the frame data into the source video file. This
	// can be called multiple times during the lifetime of the Codec.
	Encode() error
	// GetFrame returns the ith video frame.
	GetFrame(i int) (Frame, error)
	// WriteFrame overwrites the ith video frame of the intermidate video with the
	// one provided. Note that this won't actually trigger a write of the source
	// video with the new frame data, you should call Encode if you want this.
	WriteFrame(i int, f Frame) error
	// Close closes the Codec.
	Close()
}

// Frame defines the interface for a Frame object.
type Frame interface {
}

// motionJPEGCodec uses FFMPEG to decode ~any video into a sequence of JPEG
// images where we can embed data.
type motionJPEGCodec struct {
	filePath string
}

// Decode converts the source video file to a sequence of JPEG images via
// FFMPEG and stores them in a tempory directory.
func (c *motionJPEGCodec) Decode() error {
	// TODO Implement me.
	return nil
}

// Encode converts the sequence of JPEG images to a motion JPEG video
// overwriting the original source video.
func (c *motionJPEGCodec) Encode() error {
	// TODO Implement me.
	return nil
}

// GetFrame returns the ith frame.
func (c *motionJPEGCodec) GetFrame(i int) (Frame, error) {
	// TODO Implement me.
	return nil, nil
}

// WriteFrame overwrites the ith frame of the intermediate video with the
// provided frame.
func (c *motionJPEGCodec) WriteFrame(i int, f Frame) error {
	// TODO Implement me.
	return nil
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
