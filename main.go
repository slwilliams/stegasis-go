// Stagasis provides steganographic embeding of data within video files as a file system.
package main

import (
	"fmt"
	"os"

	"stegasis/filesystem"
	"stegasis/video"

	"github.com/billziss-gh/cgofuse/fuse"
)

func main() {
	codec, err := video.NewMotionJPEGCodec("")
	if err != nil {
		fmt.Printf("Failed to create new motion jpeg codec: %v", err)
		os.Exit(1)
	}
	fs := filesystem.New(codec)

	host := fuse.NewFileSystemHost(fs)
	host.Mount("X:", os.Args[1:])
}
