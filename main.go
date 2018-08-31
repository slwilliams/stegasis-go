// Stagasis provides steganographic embeding of data within video files as a file system.
// Usage: stegasis --framerate=25 path\to\video.mp4
package main

import (
	"flag"
	"fmt"
	"os"

	"stegasis/filesystem"
	"stegasis/video"

	"github.com/billziss-gh/cgofuse/fuse"
)

var (
	frameRate = flag.Int("framerate", 0, "Frame rate of the input video, if known.")
)

func main() {
	flag.Parse()

	codec, err := video.NewMotionJPEGCodec(flag.Args()[0], video.MotionJPEGCodecOptions{
		FrameRate: *frameRate,
	})
	if err != nil {
		fmt.Printf("Failed to create new motion jpeg codec: %v", err)
		os.Exit(1)
	}

	fs := filesystem.New(codec)
	host := fuse.NewFileSystemHost(fs)
	host.Mount("X:", flag.Args()[1:])
}
