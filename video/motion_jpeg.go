package video

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"sync"
	"time"

	"stegasis/image/jpeg"
)

// motionJPEGCodec uses FFMPEG to decode ~any video into a sequence of JPEG
// images where we can embed data. motionJPEGCodec implements the Codec interface.
type motionJPEGCodec struct {
	filePath string
	opts     MotionJPEGCodecOptions
	frames   []*jpeg.JPEG
}

// MotionJPEGCodecOptions holds options for the motion jpect codec.
type MotionJPEGCodecOptions struct {
	// FrameRate of the input video.
	FrameRate int
}

// Decode converts the source video file to a sequence of JPEG images via
// FFMPEG and stores them in a tempory directory.
func (c *motionJPEGCodec) Decode() error {
	tempDir := fmt.Sprintf("%s\\stegasis", os.TempDir())
	os.RemoveAll(tempDir)
	if err := os.Mkdir(tempDir, 0777); err != nil {
		return fmt.Errorf("Failed to create temp directory: %v", err)
	}
	outputPattern := fmt.Sprintf("%s\\image-%%d.jpeg", tempDir)

	fmt.Printf("Extracting video frames from %q to: %q ...\n", c.filePath, outputPattern)
	args := []string{
		"-v", "quiet",
		"-stats",
		"-r", "25",
		"-i", c.filePath,
		"-qscale:v", "2",
		"-f", "image2",
		outputPattern,
	}
	if err := exec.Command("ffmpeg", args...).Run(); err != nil {
		return fmt.Errorf("Failed to exec ffmpeg: %v", err)
	}
	fmt.Println("Successfully extracted video frames!")

	files, err := ioutil.ReadDir(tempDir)
	if err != nil {
		return fmt.Errorf("Failed to read dir %q: %v", tempDir, err)
	}

	fmt.Println("Decoding frame data...")
	now := time.Now()
	var (
		wg        sync.WaitGroup
		mux       sync.Mutex
		decodeErr error
	)
	frames := map[int]*jpeg.JPEG{}
	for i, f := range files {
		wg.Add(1)
		go func() {
			defer wg.Done()
			i := i
			f := f

			r, err := os.Open(tempDir + "\\" + f.Name())
			if err != nil {
				decodeErr = fmt.Errorf("Failed to read file %q: %v", f.Name(), err)
				return
			}

			j, err := jpeg.DecodeJPEG(r)
			if err != nil {
				decodeErr = fmt.Errorf("Failed to decode JPEG %q: %v", f.Name(), err)
				return
			}

			mux.Lock()
			frames[i] = j
			mux.Unlock()
		}()
	}
	wg.Wait()

	if decodeErr != nil {
		return decodeErr
	}

	for _, f := range frames {
		c.frames = append(c.frames, f)
	}

	fmt.Printf("Finished decoding frame data. Took: %s\n", time.Since(now))

	fmt.Println("Writing back 1 frame!!")
	f, err := os.Create(fmt.Sprintf("%s\\out.jpeg", os.TempDir()))
	if err != nil {
		fmt.Printf("failed to make finle: %v", err)
	}
	err = c.frames[0].Encode(f)
	if err != nil {
		fmt.Printf("err: %v", err)
	}
	f.Close()
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
	jf := f.(*jpeg.JPEG)
	c.frames[i] = jf
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
func NewMotionJPEGCodec(path string, opts MotionJPEGCodecOptions) (Codec, error) {
	codec := &motionJPEGCodec{
		filePath: path,
		opts:     opts,
	}
	return codec, codec.Decode()
}
