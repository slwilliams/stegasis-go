package jpeg

import (
	"bufio"
	"os"
)

// encode actually does the encoding work.
func (jp *JPEG) encode(f *os.File) error {
	bw := bufio.NewWriter(f)

	buff := make([]byte, 1024)

	// Write the Start Of Image marker.
	buff[0] = 0xff
	buff[1] = 0xd8
	bw.Write(buff[:2])

	// Write the quantization tables.
	markerlen := 2 + int(1)*(1+blockSize)
	writeMarkerHeader(bw, dqtMarker, markerlen, buff)
	for i := range jp.quant {
		bw.WriteByte(uint8(i))
		bw.Write(jp.quant[i][:])
	}

	// Write the image dimensions.
	markerlen = 8 + 3*3
	writeMarkerHeader(bw, sof0Marker, markerlen, buff)
	buff[0] = 8 // 8-bit color.
	buff[1] = uint8(jp.height >> 8)
	buff[2] = uint8(jp.height & 0xff)
	buff[3] = uint8(jp.width >> 8)
	buff[4] = uint8(jp.width & 0xff)
	buff[5] = uint8(3)
	for i := 0; i < 3; i++ {
		buff[3*i+6] = uint8(i + 1)
		// We use 4:2:0 chroma subsampling.
		buff[3*i+7] = "\x22\x11\x11"[i]
		buff[3*i+8] = "\x00\x00\x00"[i]
	}

	bw.Write(buff[:3*(3-1)+9])

	// Write the Huffman tables.
	markerlen = 2
	specs := theHuffmanSpec[:]
	for _, s := range specs {
		markerlen += 1 + 16 + len(s.value)
	}
	writeMarkerHeader(bw, dhtMarker, markerlen, buff)
	for i, s := range specs {
		bw.WriteByte("\x00\x10\x01\x11"[i])
		bw.Write(s.count[:])
		bw.Write(s.value)
	}

	// Write the image data.
	bw.Write(sosHeaderYCbCr)

	//var prevDCY, prevDCCb, prevDCCr int32
	for i, b := range jp.blocks {
		if jp.compIndex[i] == 0 {
			_ = jp.writeBlock(bw, &b, 0, 0)
		}
		if jp.compIndex[i] == 1 {
			_ = jp.writeBlock(bw, &b, 1, 0)
		}
		if jp.compIndex[i] == 2 {
			_ = jp.writeBlock(bw, &b, 1, 0)
		}
	}
	jp.emit(bw, 0x7f, 7)

	// Write the End Of Image marker.
	buff[0] = 0xff
	buff[1] = 0xd9
	bw.Write(buff[:2])
	return bw.Flush()
}

func writeMarkerHeader(bw *bufio.Writer, marker uint8, markerlen int, buff []byte) {
	buff[0] = 0xff
	buff[1] = marker
	buff[2] = uint8(markerlen >> 8)
	buff[3] = uint8(markerlen & 0xff)
	bw.Write(buff[:4])
}

func (j *JPEG) emit(bw *bufio.Writer, bits, nBits uint32) {
	nBits += j.eNBits
	bits <<= 32 - nBits
	bits |= j.eBits
	for nBits >= 8 {
		b := uint8(bits >> 24)
		bw.WriteByte(b)
		if b == 0xff {
			bw.WriteByte(0x00)
		}
		bits <<= 8
		nBits -= 8
	}
	j.eBits, j.eNBits = bits, nBits
}

func (jp *JPEG) writeBlock(bw *bufio.Writer, b *block, q quantIndex, prevDC int32) int32 {
	// Emit the DC delta.
	//dc := div(b[0], 8*int32(jp.quant[q][0]))
	dc := b[0]
	jp.emitHuffRLE(bw, huffIndex(2*q+0), 0, dc-prevDC)
	// Emit the AC components.
	h, runLength := huffIndex(2*q+1), int32(0)
	for zig := 1; zig < blockSize; zig++ {
		//ac := div(b[unzig[zig]], 8*int32(jp.quant[q][zig]))
		ac := b[unzig[zig]]
		//ac := b[zig]
		if ac == 0 {
			runLength++
		} else {
			for runLength > 15 {
				jp.emitHuff(bw, h, 0xf0)
				runLength -= 16
			}
			jp.emitHuffRLE(bw, h, runLength, ac)
			runLength = 0
		}
	}
	if runLength > 0 {
		jp.emitHuff(bw, h, 0x00)
	}
	return dc
}

// emitHuffRLE emits a run of runLength copies of value encoded with the given
// Huffman encoder.
func (jp *JPEG) emitHuffRLE(bw *bufio.Writer, h huffIndex, runLength, value int32) {
	a, b := value, value
	if a < 0 {
		a, b = -value, value-1
	}
	var nBits uint32
	if a < 0x100 {
		nBits = uint32(bitCount[a])
	} else {
		nBits = 8 + uint32(bitCount[a>>8])
	}
	jp.emitHuff(bw, h, runLength<<4|int32(nBits))
	if nBits > 0 {
		jp.emit(bw, uint32(b)&(1<<nBits-1), nBits)
	}
}

// emitHuff emits the given value with the given Huffman encoder.
func (jp *JPEG) emitHuff(bw *bufio.Writer, h huffIndex, value int32) {
	x := theHuffmanLUT[h][value]
	jp.emit(bw, x&(1<<24-1), x>>24)
}

func (h *huffmanLUT) init(s huffmanSpec) {
	maxValue := 0
	for _, v := range s.value {
		if int(v) > maxValue {
			maxValue = int(v)
		}
	}
	*h = make([]uint32, maxValue+1)
	code, k := uint32(0), 0
	for i := 0; i < len(s.count); i++ {
		nBits := uint32(i+1) << 24
		for j := uint8(0); j < s.count[i]; j++ {
			(*h)[s.value[k]] = nBits | code
			code++
			k++
		}
		code <<= 1
	}
}
