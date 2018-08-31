package video

import (
	"fmt"
	"os"
	"os/exec"
)

// motionJPEGCodec uses FFMPEG to decode ~any video into a sequence of JPEG
// images where we can embed data. motionJPEGCodec implements the Codec interface.
type motionJPEGCodec struct {
	filePath string
	frames   []Frame
}

// Decode converts the source video file to a sequence of JPEG images via
// FFMPEG and stores them in a tempory directory.
func (c *motionJPEGCodec) Decode() error {
	tempDir := fmt.Sprintf("%s\\stegasis", os.TempDir())
	if err := os.Mkdir(tempDir, 0777); err != nil {
		return fmt.Errorf("Failed to create temp directory: %v", err)
	}
	outputPattern := fmt.Sprintf("%s\\image-%%d.jpeg", tempDir)

	fmt.Println("Extracting video frames to: %q ...", outputPattern)
	args := []string{"-v", "quiet", "-stats", "-r", "25", "-i", c.filePath, "-qscale:v", "2", "-f", "image2", outputPattern}
	if err := exec.Command("ffmpeg", args...).Run(); err != nil {
		return fmt.Errorf("Failed to exec ffmpeg: %v", err)
	}
	fmt.Println("Successfully extracted video frames!")

	return nil
}

// Encode converts the sequence of JPEG images to a motion JPEG video
// overwriting the original source video.
func (c *motionJPEGCodec) Encode() error {
	for _, f := range c.frames {
		if f.IsDirty() {
			// TODO: Write this frame out.
		}
	}
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

// jpegFrame holds the data of a single JPEG frame (i.e. a single jpeg image).
// It implements the Frame interface.
type jpegFrame struct {
	dirty bool
}

// Size returns the number of elements (i.e. DCT Coefficients) which can be
// embedded in within the frame.
func (f *jpegFrame) Size() int {
	// TODO implement me.
	return 0
}

// GetElement returns the ith frame element (i.e. DCT Coefficient). Panics if
// i >= Size() or i < 0.
func (f *jpegFrame) GetElement(i int) *int {
	// TODO implement me.
	var ret int
	return &ret
}

// SetDirty marks the frame as dirty.
func (f *jpegFrame) SetDirty() {
	f.dirty = true
}

// IsDirty returns true iff the frame has been marked dirty.
func (f *jpegFrame) IsDirty() bool {
	return f.dirty
}

// NewMotionJPEGCodec returns a new motion JPEG codec.
func NewMotionJPEGCodec(path string) (Codec, error) {
	codec := &motionJPEGCodec{
		filePath: path,
	}
	return codec, codec.Decode()
}
