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

	codec := video.NewMotionJPEGCodec(flag.Args()[0], video.MotionJPEGCodecOptions{
		FrameRate: *frameRate,
	})
	if err := codec.Decode(); err != nil {
		fmt.Printf("Codec failed to decode: %v", err)
		os.Exit(1)
	}

	fs := filesystem.New(codec)
	host := fuse.NewFileSystemHost(fs)
	host.Mount("X:", flag.Args()[1:])
}
