// Package filesystem implements the FUSE filesystem operations.
package filesystem

import (
	"stegasis/video"

	"github.com/billziss-gh/cgofuse/fuse"
)

const (
	filename = "hello"
	contents = "hello, world\n"
)

// fs implements the FUSE filesystem.
type fs struct {
	fuse.FileSystemBase
	codec video.Codec
}

func (f *fs) Open(path string, flags int) (int, uint64) {
	switch path {
	case "/" + filename:
		return 0, 0
	default:
		return -fuse.ENOENT, ^uint64(0)
	}
	return 0, 0
}

func (f *fs) Getattr(path string, stat *fuse.Stat_t, fh uint64) int {
	switch path {
	case "/":
		stat.Mode = fuse.S_IFDIR | 0555
		return 0
	case "/" + filename:
		stat.Mode = fuse.S_IFREG | 0444
		stat.Size = int64(len(contents))
		return 0
	default:
		return -fuse.ENOENT
	}
	return 0
}

func (f *fs) Read(path string, buff []byte, ofst int64, fh uint64) int {
	endofst := ofst + int64(len(buff))
	if endofst > int64(len(contents)) {
		endofst = int64(len(contents))
	}
	if endofst < ofst {
		return 0
	}
	return copy(buff, contents[ofst:endofst])
}

func (f *fs) Readdir(path string, fill func(name string, stat *fuse.Stat_t, ofst int64) bool, ofst int64, fh uint64) int {
	fill(".", nil, 0)
	fill("..", nil, 0)
	fill(filename, nil, 0)
	return 0
}

// New returns a new fs object which implements fuse.FileSystemInterface.
func New(codec video.Codec) *fs {
	return &fs{
		codec: codec,
	}
}
