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
}

func DecodeJPEG(r io.Reader) (*JPEG, error) {
	buff := make([]byte, 1024)
	if _, err := r.Read(buff[:2]); err != nil {
		return nil, err
	}
	if buff[0] != 0xff || buff[1] != soiMarker {
		return nil, fmt.Errorf("missing SOI marker at start of file")
	}

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
			if err := processSOF(r, n, buff); err != nil {
				return nil, err
			}
		case sof1Marker, sof2Marker:
			return nil, fmt.Errorf("unsupported sof1, sof2 markers")
		case dhtMarker:
			if err := processDHT(r, n, buff); err != nil {
				return nil, err
			}
		case dqtMarker:
			if err := processDQT(r, n, buff); err != nil {
				return nil, err
			}
		case sosMarker:
			if err := processSOS(r, n, buff); err != nil {
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
	return nil, nil
}

func processSOF(r io.Reader, n int, buff []byte) error {
	fmt.Println("processSOF")
	if n != (6 + 3*4) {
		// 3 components.
		return fmt.Errorf("Only support YCbCr or RGB images")
	}
	return nil
}

func processDQT(r io.Reader, n int, buff []byte) error {
	fmt.Println("processDQT")
	// Just ignore since we don't care about this data.
	if _, err := r.Read(buff[:n]); err != nil {
		return err
	}
	return nil
}

func processDHT(r io.Reader, n int, buff []byte) error {
	// I think we need this to decode the SOS where the coefficients are...
	fmt.Println("processDHT")
	if _, err := r.Read(buff[:n]); err != nil {
		return err
	}
	return nil
}

func processSOS(r io.Reader, n int, buff []byte) error {
	fmt.Println("processSOS")
	// Ignore for now, but this is where the data actually is.
	if _, err := r.Read(buff[:n]); err != nil {
		return err
	}
	return nil
}
