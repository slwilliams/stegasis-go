// My re-write of a JPEG "decoder" giving me access to the raw DCT coefficients.
package jpeg

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

const (
	sof0Marker  = 0xc0 // Start Of Frame (Baseline Sequential).
	sof1Marker  = 0xc1 // Start Of Frame (Extended Sequential).
	sof2Marker  = 0xc2 // Start Of Frame (Progressive).
	dhtMarker   = 0xc4 // Define Huffman Table.
	rst0Marker  = 0xd0 // ReSTart (0).
	rst7Marker  = 0xd7 // ReSTart (7).
	soiMarker   = 0xd8 // Start Of Image.
	eoiMarker   = 0xd9 // End Of Image.
	sosMarker   = 0xda // Start Of Scan.
	dqtMarker   = 0xdb // Define Quantization Table.
	driMarker   = 0xdd // Define Restart Interval.
	comMarker   = 0xfe // COMment.
	app0Marker  = 0xe0
	app14Marker = 0xee
	app15Marker = 0xef
)

// unzig maps from the zig-zag ordering to the natural ordering. For example,
// unzig[3] is the column and row of the fourth element in zig-zag order. The
// value is 16, which means first column (16%8 == 0) and third row (16/8 == 2).
var unzig = [blockSize]int{
	0, 1, 8, 16, 9, 2, 3, 10,
	17, 24, 32, 25, 18, 11, 4, 5,
	12, 19, 26, 33, 40, 48, 41, 34,
	27, 20, 13, 6, 7, 14, 21, 28,
	35, 42, 49, 56, 57, 50, 43, 36,
	29, 22, 15, 23, 30, 37, 44, 51,
	58, 59, 52, 45, 38, 31, 39, 46,
	53, 60, 61, 54, 47, 55, 62, 63,
}

const (
	dcTable = 0
	acTable = 1
	maxTc   = 1
	maxTh   = 3
	maxTq   = 3

	maxComponents = 4
)

// Component specification, specified in section B.2.2.
type component struct {
	h  int   // Horizontal sampling factor.
	v  int   // Vertical sampling factor.
	c  uint8 // Component identifier.
	tq uint8 // Quantization table destination selector.
}

const blockSize = 64 // A DCT block is 8x8.

type block [blockSize]int32

// bits holds the unprocessed bits that have been taken from the byte-stream.
// The n least significant bits of a form the unread bits, to be read in MSB to
// LSB order.
type bits struct {
	a uint32 // accumulator.
	m uint32 // mask. m==1<<(n-1) when n>0, with m==0 when n==0.
	n int32  // the number of unread bits in a.
}

// JPEG holds a JPEG file and implements thr Frame interface.
type JPEG struct {
	path string

	height int
	width  int
	ri     int

	comps [3]component
	huffs [2][4]*huffman
	bits  bits

	// The actual DCT coefficients we embed in.
	blocks    []block
	compIndex []int
	dirty     bool

	// Encoding related fields

	// quant is the scaled quantization tables, in zig-zag order.
	quant         [1][blockSize]byte
	eBits, eNBits uint32
}

// Size returns the total number of DCT coefficients in all blocks.
func (j *JPEG) Size() int {
	return len(j.blocks) * 64
}

// GetElement returns the ith DCT coefficient.
func (j *JPEG) GetElement(i int) int {
	if i < 0 {
		panic(fmt.Errorf("JPEG GetElement i < 0: %d", i))
	}
	if i >= j.Size() {
		panic(fmt.Errorf("JPEG GetElement i >= Size(). Size: %d, i: %d", j.Size(), i))
	}

	return (int)(j.blocks[i/64][i%64])

	panic(fmt.Errorf("Reached bottom of GetElement, this should never happen"))
}

// SetElement sets the ith DCT coefficient to val.
func (j *JPEG) SetElement(i, val int) {
	if i < 0 {
		panic(fmt.Errorf("JPEG GetElement i < 0: %d", i))
	}
	if i >= j.Size() {
		panic(fmt.Errorf("JPEG GetElement i >= Size(). Size: %d, i: %d", j.Size(), i))
	}

	j.blocks[i/64][i%64] = (int32)(val)
	j.dirty = true
}

func (j *JPEG) IsDirty() bool {
	return j.dirty
}

// DecodeJPEG attempts to decode the given reader as JPEG data giving access to
// the raw DCT coefficients.
func DecodeJPEG(r io.Reader, path string) (*JPEG, error) {
	buff := make([]byte, 1024)
	if _, err := r.Read(buff[:2]); err != nil {
		return nil, err
	}
	if buff[0] != 0xff || buff[1] != soiMarker {
		return nil, fmt.Errorf("missing SOI marker at start of file")
	}

	j := &JPEG{
		path: path,
	}

	for {
		//fmt.Println("Top")
		if _, err := r.Read(buff[:2]); err != nil {
			return nil, err
		}

		if buff[0] != 0xff {
			return nil, fmt.Errorf("expected 0xff")
		}

		marker := buff[1]
		if marker == 0 {
			continue
		}

		for marker == 0xff {
			//fmt.Println("strip 0xff")
			if _, err := r.Read(buff[:1]); err != nil {
				return nil, err
			}
			marker = buff[0]
		}

		if marker == eoiMarker {
			break
		}

		if _, err := r.Read(buff[:2]); err != nil {
			return nil, err
		}
		n := int(buff[0])<<8 + int(buff[1]) - 2
		if n < 0 {
			return nil, fmt.Errorf("short segment length")
		}

		switch marker {
		case sof0Marker:
			// This is baseline, non-progressive.
			if err := j.processSOF(r, n, buff); err != nil {
				return nil, err
			}
		case sof1Marker, sof2Marker:
			return nil, fmt.Errorf("unsupported sof1, sof2 markers")
		case dhtMarker:
			if err := j.processDHT(r, n, buff); err != nil {
				return nil, err
			}
		case dqtMarker:
			if err := j.processDQT(r, n, buff); err != nil {
				return nil, err
			}
		case sosMarker:
			if err := j.processSOS(r, n, buff); err != nil {
				return nil, err
			}
		case driMarker:
			if err := j.processDRI(r, n, buff); err != nil {
				return nil, err
			}
		default:
			if app0Marker <= marker && marker <= app15Marker || marker == comMarker {
				// ignore n bytes
				if _, err := r.Read(buff[:n]); err != nil {
					return nil, err
				}
			} else if marker < 0xc0 {
				return nil, fmt.Errorf("unknown marker: %02x", marker)
			} else {
				return nil, fmt.Errorf("bad marker: %02x", marker)
			}
		}
	}

	return j, nil
}

func (j *JPEG) processDRI(r io.Reader, n int, buff []byte) error {
	if n != 2 {
		return fmt.Errorf("DRI has wrong length")
	}
	if _, err := r.Read(buff[:2]); err != nil {
		return err
	}
	j.ri = int(buff[0])<<8 + int(buff[1])
	return nil
}

func (j *JPEG) processSOF(r io.Reader, n int, buff []byte) error {
	//fmt.Println("processSOF")
	if n != (6 + 3*3) {
		// 3 components.
		return fmt.Errorf("Only support YCbCr / RGB images")
	}
	if _, err := r.Read(buff[:n]); err != nil {
		return err
	}
	// We only support 8-bit precision.
	if buff[0] != 8 {
		return fmt.Errorf("Only support 8-but precision")
	}
	height := int(buff[1])<<8 + int(buff[2])
	width := int(buff[3])<<8 + int(buff[4])
	//fmt.Printf("height: %d, width: %d\n", height, width)
	if int(buff[5]) != 3 {
		return fmt.Errorf("SOF has wrong length")
	}

	comps := [3]component{}

	for i := 0; i < 3; i++ {
		comp := component{}
		comp.c = buff[6+3*i]
		comp.tq = buff[8+3*i]

		hv := buff[7+3*i]
		h, v := int(hv>>4), int(hv&0x0f)

		comp.h = h
		comp.v = v

		comps[i] = comp
	}

	//fmt.Printf("%+v\n", comps)

	j.height = height
	j.width = width
	j.comps = comps
	return nil
}

func (jp *JPEG) processDQT(r io.Reader, n int, buff []byte) error {
	//fmt.Println("processDQT")
loop:
	for n > 0 {
		n--
		if _, err := r.Read(buff[:1]); err != nil {
			return err
		}
		x := buff[0]
		tq := x & 0x0f
		if tq > maxTq {
			return fmt.Errorf("bad Tq value")
		}
		switch x >> 4 {
		default:
			return fmt.Errorf("bad Pq value")
		case 0:
			if n < blockSize {
				break loop
			}
			n -= blockSize
			if _, err := r.Read(buff[:blockSize]); err != nil {
				return err
			}
			for i := range jp.quant[tq] {
				jp.quant[tq][i] = buff[i]
			}
		case 1:
			if n < 2*blockSize {
				break loop
			}
			n -= 2 * blockSize
			if _, err := r.Read(buff[:2*blockSize]); err != nil {
				return err
			}
			for i := range jp.quant[tq] {
				//jp.quant[tq][i] = int32(buff[2*i])<<8 | int32(buff[2*i+1])
				jp.quant[tq][i] = buff[2*i+1]
			}
		}
	}
	if n != 0 {
		return fmt.Errorf("DQT has wrong length")
	}
	return nil
}

func (j *JPEG) processDHT(r io.Reader, n int, buff []byte) error {
	//fmt.Println("processDHT")

	for n > 0 {
		if n < 17 {
			return fmt.Errorf("DHT has wrong length")
		}
		if _, err := r.Read(buff[:17]); err != nil {
			return err
		}
		tc := buff[0] >> 4
		if tc > maxTc {
			return fmt.Errorf("bad Tc value")
		}
		th := buff[0] & 0x0f
		if th > 1 {
			return fmt.Errorf("bad Th value")
		}
		h := huffman{}

		// Read nCodes and h.vals (and derive h.nCodes).
		// nCodes[i] is the number of codes with code length i.
		// h.nCodes is the total number of codes.
		h.nCodes = 0
		var nCodes [maxCodeLength]int32
		for i := range nCodes {
			nCodes[i] = int32(buff[i+1])
			h.nCodes += nCodes[i]
		}
		if h.nCodes == 0 {
			return fmt.Errorf("Huffman table has zero length")
		}
		if h.nCodes > maxNCodes {
			return fmt.Errorf("Huffman table has excessive length")
		}
		n -= int(h.nCodes) + 17
		if n < 0 {
			return fmt.Errorf("DHT has wrong length")
		}
		if _, err := r.Read(h.vals[:h.nCodes]); err != nil {
			return err
		}

		// Derive the look-up table.
		for i := range h.lut {
			h.lut[i] = 0
		}
		var x, code uint32
		for i := uint32(0); i < lutSize; i++ {
			code <<= 1
			for j := int32(0); j < nCodes[i]; j++ {
				// The codeLength is 1+i, so shift code by 8-(1+i) to
				// calculate the high bits for every 8-bit sequence
				// whose codeLength's high bits matches code.
				// The high 8 bits of lutValue are the encoded value.
				// The low 8 bits are 1 plus the codeLength.
				base := uint8(code << (7 - i))
				lutValue := uint16(h.vals[x])<<8 | uint16(2+i)
				for k := uint8(0); k < 1<<(7-i); k++ {
					h.lut[base|k] = lutValue
				}
				code++
				x++
			}
		}

		// Derive minCodes, maxCodes, and valsIndices.
		var c, index int32
		for i, n := range nCodes {
			if n == 0 {
				h.minCodes[i] = -1
				h.maxCodes[i] = -1
				h.valsIndices[i] = -1
			} else {
				h.minCodes[i] = c
				h.maxCodes[i] = c + n - 1
				h.valsIndices[i] = index
				c += n
				index += n
			}
			c <<= 1
		}

		j.huffs[tc][th] = &h
	}

	return nil
}

// ensureNBits reads bytes from the byte buffer to ensure that d.bits.n is at
// least n. For best performance (avoiding function calls inside hot loops),
// the caller is the one responsible for first checking that d.bits.n < n.
func (j *JPEG) ensureNBits(r io.Reader, n int32) error {
	for {
		c, err := j.readByteStuffedByte(r)
		if err != nil {
			return err
		}
		j.bits.a = j.bits.a<<8 | uint32(c)
		j.bits.n += 8
		if j.bits.m == 0 {
			j.bits.m = 1 << 7
		} else {
			j.bits.m <<= 8
		}
		if j.bits.n >= n {
			break
		}
	}
	return nil
}

func (jp *JPEG) processSOS(r io.Reader, n int, buff []byte) error {
	//fmt.Println("processSOS")

	if _, err := r.Read(buff[:n]); err != nil {
		return err
	}
	nComp := int(buff[0])
	if n != 4+2*nComp {
		return fmt.Errorf("SOS length inconsistent with number of components")
	}
	var scan [3]struct {
		compIndex uint8
		td        uint8 // DC table selector.
		ta        uint8 // AC table selector.
	}
	totalHV := 0
	for i := 0; i < nComp; i++ {
		cs := buff[1+2*i] // Component selector.
		compIndex := -1
		for j, comp := range jp.comps[:3] {
			if cs == comp.c {
				compIndex = j
			}
		}
		if compIndex < 0 {
			return fmt.Errorf("unknown component selector")
		}
		scan[i].compIndex = uint8(compIndex)
		totalHV += jp.comps[compIndex].h * jp.comps[compIndex].v

		// The baseline t <= 1 restriction is specified in table B.3.
		scan[i].td = buff[2+2*i] >> 4
		if t := scan[i].td; t > maxTh || t > 1 {
			return fmt.Errorf("bad Td value")
		}
		scan[i].ta = buff[2+2*i] & 0x0f
		if t := scan[i].ta; t > maxTh || t > 1 {
			return fmt.Errorf("bad Ta value")
		}
	}
	// zigStart and zigEnd are the spectral selection bounds.
	// ah and al are the successive approximation high and low values.
	// The spec calls these values Ss, Se, Ah and Al.
	//
	// For sequential JPEGs, these parameters are hard-coded to 0/63/0/0, as
	// per table B.3.
	zigStart, zigEnd := int32(0), int32(63)

	// mxx and myy are the number of MCUs (Minimum Coded Units) in the image.
	h0, v0 := jp.comps[0].h, jp.comps[0].v // The h and v values from the Y components.
	mxx := (jp.width + 8*h0 - 1) / (8 * h0)
	myy := (jp.height + 8*v0 - 1) / (8 * v0)

	jp.bits = bits{}
	mcu, expectedRST := 0, uint8(rst0Marker)
	var (
		// b is the decoded coefficients, in natural (not zig-zag) order.
		b  block
		dc [3]int32
		// bx and by are the location of the current block, in units of 8x8
		// blocks: the third block in the first row has (bx, by) = (2, 0).
		bx, by     int
		blockCount int
		eobRun     uint16
	)
	for my := 0; my < myy; my++ {
		for mx := 0; mx < mxx; mx++ {
			for i := 0; i < nComp; i++ {
				compIndex := scan[i].compIndex
				hi := jp.comps[compIndex].h
				vi := jp.comps[compIndex].v
				for j := 0; j < hi*vi; j++ {
					// The blocks are traversed one MCU at a time. For 4:2:0 chroma
					// subsampling, there are four Y 8x8 blocks in every 16x16 MCU.
					//
					// For a sequential 32x16 pixel image, the Y blocks visiting order is:
					//  0 1 4 5
					//  2 3 6 7
					//
					// For progressive images, the interleaved scans (those with nComp > 1)
					// are traversed as above, but non-interleaved scans are traversed left
					// to right, top to bottom:
					//  0 1 2 3
					//  4 5 6 7
					// Only DC scans (zigStart == 0) can be interleaved. AC scans must have
					// only one component.
					//
					// To further complicate matters, for non-interleaved scans, there is no
					// data for any blocks that are inside the image at the MCU level but
					// outside the image at the pixel level. For example, a 24x16 pixel 4:2:0
					// progressive image consists of two 16x16 MCUs. The interleaved scans
					// will process 8 Y blocks:
					//  0 1 4 5
					//  2 3 6 7
					// The non-interleaved scans will process only 6 Y blocks:
					//  0 1 2
					//  3 4 5
					if nComp != 1 {
						bx = hi*mx + j%hi
						by = vi*my + j/hi
					} else {
						q := mxx * hi
						bx = blockCount % q
						by = blockCount / q
						blockCount++
						if bx*8 >= jp.width || by*8 >= jp.height {
							continue
						}
					}

					b = block{}
					zig := zigStart
					if zig == 0 {
						zig++
						// Decode the DC coefficient, as specified in section F.2.2.1.
						value, err := jp.decodeHuffman(r, jp.huffs[dcTable][scan[i].td])
						if err != nil {
							return err
						}
						if value > 16 {
							return fmt.Errorf("excessive DC component")
						}
						dcDelta, err := jp.receiveExtend(r, value)
						if err != nil {
							return err
						}
						dc[compIndex] += dcDelta
						b[0] = dcDelta
					}

					if zig <= zigEnd && eobRun > 0 {
						eobRun--
					} else {
						// Decode the AC coefficients, as specified in section F.2.2.2.
						huff := jp.huffs[acTable][scan[i].ta]
						for ; zig <= zigEnd; zig++ {
							value, err := jp.decodeHuffman(r, huff)
							if err != nil {
								return err
							}
							val0 := value >> 4
							val1 := value & 0x0f
							if val1 != 0 {
								zig += int32(val0)
								if zig > zigEnd {
									break
								}
								ac, err := jp.receiveExtend(r, val1)
								if err != nil {
									return err
								}
								b[unzig[zig]] = ac
							} else {
								if val0 != 0x0f {
									eobRun = uint16(1 << val0)
									if val0 != 0 {
										bits, err := jp.decodeBits(r, int32(val0))
										if err != nil {
											return err
										}
										eobRun |= uint16(bits)
									}
									eobRun--
									break
								}
								zig += 0x0f
							}
						}
					}
					jp.blocks = append(jp.blocks, b)
					jp.compIndex = append(jp.compIndex, int(compIndex))
				} // for j
			} // for i
			mcu++
			if jp.ri > 0 && mcu%jp.ri == 0 && mcu < mxx*myy {
				// A more sophisticated decoder could use RST[0-7] markers to resynchronize from corrupt input,
				// but this one assumes well-formed input, and hence the restart marker follows immediately.
				if _, err := r.Read(buff[:2]); err != nil {
					return err
				}
				if buff[0] != 0xff || buff[1] != expectedRST {
					return fmt.Errorf("bad RST marker")
				}
				expectedRST++
				if expectedRST == rst7Marker+1 {
					expectedRST = rst0Marker
				}
				// Reset the Huffman decoder.
				jp.bits = bits{}
				// Reset the DC components, as per section F.2.1.3.1.
				dc = [3]int32{}
				// Reset the progressive decoder state, as per section G.1.2.2.
				eobRun = 0
			}
		} // for mx
	} // for my

	return nil
}

func (j *JPEG) receiveExtend(r io.Reader, t uint8) (int32, error) {
	if j.bits.n < int32(t) {
		if err := j.ensureNBits(r, int32(t)); err != nil {
			return 0, err
		}
	}
	j.bits.n -= int32(t)
	j.bits.m >>= t
	s := int32(1) << t
	x := int32(j.bits.a>>uint8(j.bits.n)) & (s - 1)
	if x < s>>1 {
		x += ((-1) << t) + 1
	}
	return x, nil
}

// readByteStuffedByte is like readByte but is for byte-stuffed Huffman data.
func (j *JPEG) readByteStuffedByte(r io.Reader) (byte, error) {
	// Take the fast path if d.bytes.buf contains at least two bytes.
	/*if d.bytes.i+2 <= d.bytes.j {
	  x = d.bytes.buf[d.bytes.i]
	  d.bytes.i++
	  d.bytes.nUnreadable = 1
	  if x != 0xff {
	    return x, err
	  }
	  if d.bytes.buf[d.bytes.i] != 0x00 {
	    return 0, errMissingFF00
	  }
	  d.bytes.i++
	  d.bytes.nUnreadable = 2
	  return 0xff, nil
	}*/

	//d.bytes.nUnreadable = 0

	tmp := make([]byte, 1, 1)
	_, err := r.Read(tmp)
	x := tmp[0]
	if err != nil {
		return 0, err
	}
	//d.bytes.nUnreadable = 1
	if x != 0xff {
		return x, nil
	}

	_, err = r.Read(tmp)
	x = tmp[0]
	if err != nil {
		return 0, err
	}
	//d.bytes.nUnreadable = 2
	if x != 0x00 {
		return 0, fmt.Errorf("missing 0xff00 sequence")
	}
	return 0xff, nil
}

func (j *JPEG) decodeBits(r io.Reader, n int32) (uint32, error) {
	if j.bits.n < n {
		if err := j.ensureNBits(r, n); err != nil {
			return 0, err
		}
	}
	ret := j.bits.a >> uint32(j.bits.n-n)
	ret &= (1 << uint32(n)) - 1
	j.bits.n -= n
	j.bits.m >>= uint32(n)
	return ret, nil
}

// decodeHuffman returns the next Huffman-coded value from the bit-stream,
// decoded according to h.
func (j *JPEG) decodeHuffman(r io.Reader, h *huffman) (uint8, error) {
	if h.nCodes == 0 {
		return 0, fmt.Errorf("uninitialized Huffman table")
	}

	/*if d.bits.n < 8 {
	    if err := d.ensureNBits(8); err != nil {
	      if err != errMissingFF00 && err != errShortHuffmanData {
	        return 0, err
	      }
	      // There are no more bytes of data in this segment, but we may still
	      // be able to read the next symbol out of the previously read bits.
	      // First, undo the readByte that the ensureNBits call made.
	      if d.bytes.nUnreadable != 0 {
	        d.unreadByteStuffedByte()
	      }
	      goto slowPath
	    }
	  }
	  if v := h.lut[(d.bits.a>>uint32(d.bits.n-lutSize))&0xff]; v != 0 {
	    n := (v & 0xff) - 1
	    d.bits.n -= int32(n)
	    d.bits.m >>= n
	    return uint8(v >> 8), nil
	  }*/

	//slowPath:
	for i, code := 0, int32(0); i < maxCodeLength; i++ {
		if j.bits.n == 0 {
			if err := j.ensureNBits(r, 1); err != nil {
				return 0, err
			}
		}
		if j.bits.a&j.bits.m != 0 {
			code |= 1
		}
		j.bits.n--
		j.bits.m >>= 1
		if code <= h.maxCodes[i] {
			return h.vals[h.valsIndices[i]+code-h.minCodes[i]], nil
		}
		code <<= 1
	}
	return 0, fmt.Errorf("bad Huffman code")
}

// maxCodeLength is the maximum (inclusive) number of bits in a Huffman code.
const maxCodeLength = 16

// maxNCodes is the maximum (inclusive) number of codes in a Huffman tree.
const maxNCodes = 256

// lutSize is the log-2 size of the Huffman decoder's look-up table.
const lutSize = 8

// huffman is a Huffman decoder, specified in section C.
type huffman struct {
	// length is the number of codes in the tree.
	nCodes int32
	// lut is the look-up table for the next lutSize bits in the bit-stream.
	// The high 8 bits of the uint16 are the encoded value. The low 8 bits
	// are 1 plus the code length, or 0 if the value is too large to fit in
	// lutSize bits.
	lut [1 << lutSize]uint16
	// vals are the decoded values, sorted by their encoding.
	vals [maxNCodes]uint8
	// minCodes[i] is the minimum code of length i, or -1 if there are no
	// codes of that length.
	minCodes [maxCodeLength]int32
	// maxCodes[i] is the maximum code of length i, or -1 if there are no
	// codes of that length.
	maxCodes [maxCodeLength]int32
	// valsIndices[i] is the index into vals of minCodes[i].
	valsIndices [maxCodeLength]int32
}

// Encode encodes the current JPEG data and writes it out as a jpeg file to the
// filePath provided when the JPEG was created (i.e. it overwrites the current
// jpeg on disk).
func (jp *JPEG) Encode() error {
	f, err := os.OpenFile(jp.path, os.O_RDWR, 0755)
	if err != nil {
		return fmt.Errorf("Could not open %q to write: %v", jp.path, err)
	}

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

// bitCount counts the number of bits needed to hold an integer.
var bitCount = [256]byte{
	0, 1, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 4,
	5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5,
	6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6,
	6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
}

type quantIndex int

const (
	quantIndexLuminance quantIndex = iota
	quantIndexChrominance
	nQuantIndex
)

// unscaledQuant are the unscaled quantization tables in zig-zag order. Each
// encoder copies and scales the tables according to its quality parameter.
// The values are derived from section K.1 after converting from natural to
// zig-zag order.
var unscaledQuant = [nQuantIndex][blockSize]byte{
	// Luminance.
	{
		16, 11, 12, 14, 12, 10, 16, 14,
		13, 14, 18, 17, 16, 19, 24, 40,
		26, 24, 22, 22, 24, 49, 35, 37,
		29, 40, 58, 51, 61, 60, 57, 51,
		56, 55, 64, 72, 92, 78, 64, 68,
		87, 69, 55, 56, 80, 109, 81, 87,
		95, 98, 103, 104, 103, 62, 77, 113,
		121, 112, 100, 120, 92, 101, 103, 99,
	},
	// Chrominance.
	{
		17, 18, 18, 24, 21, 24, 47, 26,
		26, 47, 99, 66, 56, 66, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
	},
}

type huffIndex int

const (
	huffIndexLuminanceDC huffIndex = iota
	huffIndexLuminanceAC
	huffIndexChrominanceDC
	huffIndexChrominanceAC
	nHuffIndex
)

// huffmanSpec specifies a Huffman encoding.
type huffmanSpec struct {
	// count[i] is the number of codes of length i bits.
	count [16]byte
	// value[i] is the decoded value of the i'th codeword.
	value []byte
}

// theHuffmanSpec is the Huffman encoding specifications.
// This encoder uses the same Huffman encoding for all images.
var theHuffmanSpec = [nHuffIndex]huffmanSpec{
	// Luminance DC.
	{
		[16]byte{0, 1, 5, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0},
		[]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
	},
	// Luminance AC.
	{
		[16]byte{0, 2, 1, 3, 3, 2, 4, 3, 5, 5, 4, 4, 0, 0, 1, 125},
		[]byte{
			0x01, 0x02, 0x03, 0x00, 0x04, 0x11, 0x05, 0x12,
			0x21, 0x31, 0x41, 0x06, 0x13, 0x51, 0x61, 0x07,
			0x22, 0x71, 0x14, 0x32, 0x81, 0x91, 0xa1, 0x08,
			0x23, 0x42, 0xb1, 0xc1, 0x15, 0x52, 0xd1, 0xf0,
			0x24, 0x33, 0x62, 0x72, 0x82, 0x09, 0x0a, 0x16,
			0x17, 0x18, 0x19, 0x1a, 0x25, 0x26, 0x27, 0x28,
			0x29, 0x2a, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39,
			0x3a, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49,
			0x4a, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59,
			0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69,
			0x6a, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78, 0x79,
			0x7a, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89,
			0x8a, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98,
			0x99, 0x9a, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7,
			0xa8, 0xa9, 0xaa, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6,
			0xb7, 0xb8, 0xb9, 0xba, 0xc2, 0xc3, 0xc4, 0xc5,
			0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xd2, 0xd3, 0xd4,
			0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda, 0xe1, 0xe2,
			0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9, 0xea,
			0xf1, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8,
			0xf9, 0xfa,
		},
	},
	// Chrominance DC.
	{
		[16]byte{0, 3, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0},
		[]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
	},
	// Chrominance AC.
	{
		[16]byte{0, 2, 1, 2, 4, 4, 3, 4, 7, 5, 4, 4, 0, 1, 2, 119},
		[]byte{
			0x00, 0x01, 0x02, 0x03, 0x11, 0x04, 0x05, 0x21,
			0x31, 0x06, 0x12, 0x41, 0x51, 0x07, 0x61, 0x71,
			0x13, 0x22, 0x32, 0x81, 0x08, 0x14, 0x42, 0x91,
			0xa1, 0xb1, 0xc1, 0x09, 0x23, 0x33, 0x52, 0xf0,
			0x15, 0x62, 0x72, 0xd1, 0x0a, 0x16, 0x24, 0x34,
			0xe1, 0x25, 0xf1, 0x17, 0x18, 0x19, 0x1a, 0x26,
			0x27, 0x28, 0x29, 0x2a, 0x35, 0x36, 0x37, 0x38,
			0x39, 0x3a, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48,
			0x49, 0x4a, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58,
			0x59, 0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68,
			0x69, 0x6a, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78,
			0x79, 0x7a, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87,
			0x88, 0x89, 0x8a, 0x92, 0x93, 0x94, 0x95, 0x96,
			0x97, 0x98, 0x99, 0x9a, 0xa2, 0xa3, 0xa4, 0xa5,
			0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xb2, 0xb3, 0xb4,
			0xb5, 0xb6, 0xb7, 0xb8, 0xb9, 0xba, 0xc2, 0xc3,
			0xc4, 0xc5, 0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xd2,
			0xd3, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda,
			0xe2, 0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9,
			0xea, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8,
			0xf9, 0xfa,
		},
	},
}

// huffmanLUT is a compiled look-up table representation of a huffmanSpec.
// Each value maps to a uint32 of which the 8 most significant bits hold the
// codeword size in bits and the 24 least significant bits hold the codeword.
// The maximum codeword size is 16 bits.
type huffmanLUT []uint32

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

// theHuffmanLUT are compiled representations of theHuffmanSpec.
var theHuffmanLUT [4]huffmanLUT

func init() {
	for i, s := range theHuffmanSpec {
		theHuffmanLUT[i].init(s)
	}
}

// sosHeaderYCbCr is the SOS marker "\xff\xda" followed by 12 bytes:
//  - the marker length "\x00\x0c",
//  - the number of components "\x03",
//  - component 1 uses DC table 0 and AC table 0 "\x01\x00",
//  - component 2 uses DC table 1 and AC table 1 "\x02\x11",
//  - component 3 uses DC table 1 and AC table 1 "\x03\x11",
//  - the bytes "\x00\x3f\x00". Section B.2.3 of the spec says that for
//    sequential DCTs, those bytes (8-bit Ss, 8-bit Se, 4-bit Ah, 4-bit Al)
//    should be 0x00, 0x3f, 0x00<<4 | 0x00.
var sosHeaderYCbCr = []byte{
	0xff, 0xda, 0x00, 0x0c, 0x03, 0x01, 0x00, 0x02,
	0x11, 0x03, 0x11, 0x00, 0x3f, 0x00,
}
