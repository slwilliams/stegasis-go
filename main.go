// Stagasis provides steganographic embeding of data within video files as a file system.
package main

import (
	"os"

	"stegasis/filesystem"

	"github.com/billziss-gh/cgofuse/fuse"
)

func main() {
	fs := filesystem.New()

	host := fuse.NewFileSystemHost(fs)
	host.Mount("X:", os.Args[1:])
}
