// My re-write of a JPEG "decoder" giving me access to the raw DCT coefficients.
package jpeg

import (
	"encoding/hex"
	"fmt"
	"io"
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

// JPEG holds a JPEG file.
type JPEG struct {
	height int
	width  int

	comps [3]component
	huffs [2][4]*huffman
	bits  bits
}

func DecodeJPEG(r io.Reader) (*JPEG, error) {
	buff := make([]byte, 1024)
	if _, err := r.Read(buff[:2]); err != nil {
		return nil, err
	}
	if buff[0] != 0xff || buff[1] != soiMarker {
		return nil, fmt.Errorf("missing SOI marker at start of file")
	}

	j := &JPEG{}

	for {
		fmt.Println("Top")
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
			fmt.Println("strip 0xff")
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
			return nil, FormatError("short segment length")
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
			return nil, fmt.Errorf("unsupported DRI marker")
		default:
			fmt.Println("defatul")
			if app0Marker <= marker && marker <= app15Marker || marker == comMarker {
				// ignore n bytes
				if _, err := r.Read(buff[:n]); err != nil {
					return nil, err
				}
			} else if marker < 0xc0 { // See Table B.1 "Marker code assignments".
				return nil, fmt.Errorf("unknown marker: %02x", marker)
			} else {
				return nil, fmt.Errorf("bad marker: %02x", marker)
			}
		}
	}

	fmt.Println("%s", hex.Dump(buff))
	return j, nil
}

// Component specification, specified in section B.2.2.
type component struct {
	h  int   // Horizontal sampling factor.
	v  int   // Vertical sampling factor.
	c  uint8 // Component identifier.
	tq uint8 // Quantization table destination selector.
}

func (j *JPEG) processSOF(r io.Reader, n int, buff []byte) error {
	fmt.Println("processSOF")
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
	fmt.Printf("height: %d, width: %d\n", height, width)
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

	fmt.Printf("%+v\n", comps)

	j.height = height
	j.width = width
	j.comps = comps
	return nil
}

func (j *JPEG) processDQT(r io.Reader, n int, buff []byte) error {
	fmt.Println("processDQT")
	// Just ignore since we don't care about this data.
	if _, err := r.Read(buff[:n]); err != nil {
		return err
	}
	return nil
}

func (j *JPEG) processDHT(r io.Reader, n int, buff []byte) error {
	fmt.Println("processDHT")

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
		fmt.Printf("\n%+v\n", h)
	}

	return nil
}

func (j *JPEG) processSOS(r io.Reader, n int, buff []byte) error {
	fmt.Println("processSOS")

	if _, err := d.Read(buff[:n]); err != nil {
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
		for j, comp := range j.comps[:3] {
			if cs == comp.c {
				compIndex = j
			}
		}
		if compIndex < 0 {
			return fmt.Errorf("unknown component selector")
		}
		scan[i].compIndex = uint8(compIndex)
		totalHV += j.comps[compIndex].h * j.comps[compIndex].v

		// The baseline t <= 1 restriction is specified in table B.3.
		scan[i].td = buff[2+2*i] >> 4
		if t := scan[i].td; t > maxTh || t > 1 {
			return FormatError("bad Td value")
		}
		scan[i].ta = buff[2+2*i] & 0x0f
		if t := scan[i].ta; t > maxTh || t > 1 {
			return FormatError("bad Ta value")
		}
	}
	// zigStart and zigEnd are the spectral selection bounds.
	// ah and al are the successive approximation high and low values.
	// The spec calls these values Ss, Se, Ah and Al.
	//
	// For sequential JPEGs, these parameters are hard-coded to 0/63/0/0, as
	// per table B.3.
	zigStart, zigEnd, ah, al := int32(0), int32(63), uint32(0), uint32(0)

	// mxx and myy are the number of MCUs (Minimum Coded Units) in the image.
	h0, v0 := j.comps[0].h, j.comps[0].v // The h and v values from the Y components.
	mxx := (j.width + 8*h0 - 1) / (8 * h0)
	myy := (j.height + 8*v0 - 1) / (8 * v0)

	j.bits = bits{}
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
				hi := j.comps[compIndex].h
				vi := j.comps[compIndex].v
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
						if bx*8 >= j.width || by*8 >= j.height {
							continue
						}
					}

					b = block{}
					zig := zigStart
					if zig == 0 {
						zig++
						// Decode the DC coefficient, as specified in section F.2.2.1.
						value, err := d.decodeHuffman(&d.huff[dcTable][scan[i].td])
						if err != nil {
							return err
						}
						if value > 16 {
							return UnsupportedError("excessive DC component")
						}
						dc[compIndex] = value
						/*dcDelta, err := d.receiveExtend(value)
						if err != nil {
							return err
						}
						dc[compIndex] += dcDelta*/
						b[0] = dc[compIndex]
					}

					if zig <= zigEnd && eobRun > 0 {
						eobRun--
					} else {
						// Decode the AC coefficients, as specified in section F.2.2.2.
						huff := &j.huffs[acTable][scan[i].ta]
						for ; zig <= zigEnd; zig++ {
							value, err := d.decodeHuffman(huff)
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
								/*ac, err := d.receiveExtend(val1)
								if err != nil {
									return err
								}*/
								b[unzig[zig]] = ac
							} else {
								if val0 != 0x0f {
									eobRun = uint16(1 << val0)
									if val0 != 0 {
										bits, err := d.decodeBits(int32(val0))
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
					fmt.Printf("\nBLOCK:\n%v\n", b)
				} // for j
			} // for i
			mcu++
			if d.ri > 0 && mcu%d.ri == 0 && mcu < mxx*myy {
				// A more sophisticated decoder could use RST[0-7] markers to resynchronize from corrupt input,
				// but this one assumes well-formed input, and hence the restart marker follows immediately.
				if _, err := r.Read(buff[:2]); err != nil {
					return err
				}
				if buff[0] != 0xff || buff[1] != expectedRST {
					return FormatError("bad RST marker")
				}
				expectedRST++
				if expectedRST == rst7Marker+1 {
					expectedRST = rst0Marker
				}
				// Reset the Huffman decoder.
				j.bits = bits{}
				// Reset the DC components, as per section F.2.1.3.1.
				dc = [maxComponents]int32{}
				// Reset the progressive decoder state, as per section G.1.2.2.
				eobRun = 0
			}
		} // for mx
	} // for my

	return nil
}
