package icon

import (
	"encoding/binary"
	"errors"
	"math"
	"testing"
)

func bitmapHeader(headerSize uint32, width, height int32) []byte {
	header := make([]byte, bitmapHeaderSize)
	binary.LittleEndian.PutUint32(header[0:], headerSize)
	binary.LittleEndian.PutUint32(header[4:], uint32(width))
	binary.LittleEndian.PutUint32(header[8:], uint32(height))
	binary.LittleEndian.PutUint16(header[14:], 32)
	return header
}

func TestMalformedBitmapsAreRejectedWithoutPanicking(test *testing.T) {
	examples := map[string][]byte{
		"header size beyond the data":         bitmapHeader(math.MaxUint32, 16, 32),
		"header size shorter than a header":   append(bitmapHeader(12, 1, 2), 0, 0, 0, 0),
		"dimensions that overflow the pixels": bitmapHeader(bitmapHeaderSize, math.MaxInt32, math.MaxInt32-1),
		"negative dimensions":                 bitmapHeader(bitmapHeaderSize, -16, 32),
		"missing pixels":                      bitmapHeader(bitmapHeaderSize, 16, 32),
	}
	for name, data := range examples {
		test.Run(name, func(test *testing.T) {
			if _, err := asPNG(data); !errors.Is(err, ErrMalformedIcon) {
				test.Errorf("err = %v, want ErrMalformedIcon", err)
			}
		})
	}
}
