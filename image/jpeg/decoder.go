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
	// Ignore for now, but this is where the data actually is.
	if _, err := r.Read(buff[:n]); err != nil {
		return err
	}
	return nil
}
