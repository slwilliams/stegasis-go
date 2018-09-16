package jpeg

// Component specification, specified in section B.2.2.
type component struct {
	h  int   // Horizontal sampling factor.
	v  int   // Vertical sampling factor.
	c  uint8 // Component identifier.
	tq uint8 // Quantization table destination selector.
}

type block [blockSize]int32

// bits holds the unprocessed bits that have been taken from the byte-stream.
// The n least significant bits of a form the unread bits, to be read in MSB to
// LSB order.
type bits struct {
	a uint32 // accumulator.
	m uint32 // mask. m==1<<(n-1) when n>0, with m==0 when n==0.
	n int32  // the number of unread bits in a.
}

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

type quantIndex int
type huffIndex int

// huffmanSpec specifies a Huffman encoding.
type huffmanSpec struct {
	// count[i] is the number of codes of length i bits.
	count [16]byte
	// value[i] is the decoded value of the i'th codeword.
	value []byte
}

// huffmanLUT is a compiled look-up table representation of a huffmanSpec.
// Each value maps to a uint32 of which the 8 most significant bits hold the
// codeword size in bits and the 24 least significant bits hold the codeword.
// The maximum codeword size is 16 bits.
type huffmanLUT []uint32

// theHuffmanLUT are compiled representations of theHuffmanSpec.
var theHuffmanLUT [4]huffmanLUT

func init() {
	for i, s := range theHuffmanSpec {
		theHuffmanLUT[i].init(s)
	}
}
