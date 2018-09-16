package video

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
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

	total := int32(len(files))
	c.frames = make([]*jpeg.JPEG, len(files))
	sem := make(chan struct{}, 20)
	for i, f := range files {
		f := f
		if i == 200 {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()

			jpegPath := tempDir + "\\" + f.Name()
			r, err := os.Open(jpegPath)
			if err != nil {
				decodeErr = fmt.Errorf("Failed to read file %q: %v", f.Name(), err)
				return
			}

			j, err := jpeg.DecodeJPEG(r, jpegPath)
			if err != nil {
				decodeErr = fmt.Errorf("Failed to decode JPEG %q: %v", f.Name(), err)
				return
			}

			index := strings.Split(strings.Split(f.Name(), ".")[0], "-")[1]
			i, err := strconv.Atoi(index)
			if err != nil {
				decodeErr = fmt.Errorf("Could not extract index from filename %q: %v", f.Name(), err)
				return
			}

			mux.Lock()
			c.frames[i] = j
			mux.Unlock()

			atomic.AddInt32(&total, -1)
			if total%50 == 0 {
				fmt.Printf("Frames left: %d\n", total)
			}
			<-sem
		}()
	}
	wg.Wait()

	if decodeErr != nil {
		return decodeErr
	}

	fmt.Printf("Finished decoding frame data. Took: %s\n", time.Since(now))
	return nil
}

// Encode converts the sequence of JPEG images to a motion JPEG video
// overwriting the original source video.
func (c *motionJPEGCodec) Encode() error {
	for i, f := range c.frames {
		if f.IsDirty() {
			err := f.Encode()
			if err != nil {
				return fmt.Errorf("Failed to encode frame %d: %v", i, err)
			}
		}
	}

	// TODO call FFMPEG to re-assemble the frames into a mjpeg.
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

// Close closes the motion JPEG codec.
func (c *motionJPEGCodec) Close() {
}

// NewMotionJPEGCodec returns a new motion JPEG codec.
func NewMotionJPEGCodec(path string, opts MotionJPEGCodecOptions) Codec {
	return &motionJPEGCodec{
		filePath: path,
		opts:     opts,
	}
}
