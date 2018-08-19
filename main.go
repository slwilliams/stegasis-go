// Stagasis provides steganographic embeding of data within video files as a file system.
package main

import (
	"os"

	"stegasis/filesystem"
	"stegasis/video"

	"github.com/billziss-gh/cgofuse/fuse"
)

func main() {
	codec, _ := video.NewMotionJPEGCodec("")
	fs := filesystem.New(codec)

	host := fuse.NewFileSystemHost(fs)
	host.Mount("X:", os.Args[1:])
}
