package icon

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"

	"github.com/tc-hib/winres"
)

const (
	bitmapHeaderSize = 40
	largestIconSide  = 1024
)

var (
	ErrNoIcon          = errors.New("executable has no icon")
	ErrMalformedIcon   = errors.New("malformed icon data")
	ErrUnsupportedIcon = errors.New("icon image is neither PNG nor a 32-bit bitmap")
	pngSignature       = []byte("\x89PNG\r\n\x1a\n")
)

type entry struct {
	width int
	data  []byte
}

func ExtractPNG(executable, destination string) error {
	file, err := os.Open(executable)
	if err != nil {
		return err
	}
	defer file.Close()
	resources, err := winres.LoadFromEXE(file)
	if err != nil {
		return err
	}
	picture, err := largestPNGOf(resources)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	return os.WriteFile(destination, picture, 0o644)
}

func largestPNGOf(resources *winres.ResourceSet) ([]byte, error) {
	identifier, language, found := firstIcon(resources)
	if !found {
		return nil, ErrNoIcon
	}
	group, err := resources.GetIconTranslation(identifier, language)
	if err != nil {
		return nil, err
	}
	var ico bytes.Buffer
	if err := group.SaveICO(&ico); err != nil {
		return nil, err
	}
	return largestPNG(ico.Bytes())
}

func firstIcon(resources *winres.ResourceSet) (winres.Identifier, uint16, bool) {
	var identifier winres.Identifier
	var language uint16
	found := false
	resources.WalkType(winres.RT_GROUP_ICON, func(resourceID winres.Identifier, languageID uint16, _ []byte) bool {
		identifier, language, found = resourceID, languageID, true
		return false
	})
	return identifier, language, found
}

func largestPNG(ico []byte) ([]byte, error) {
	entries, err := entriesOf(ico)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, ErrNoIcon
	}
	largest := slices.MaxFunc(entries, func(first, second entry) int { return cmp.Compare(first.width, second.width) })
	return asPNG(largest.data)
}

func asPNG(data []byte) ([]byte, error) {
	if bytes.HasPrefix(data, pngSignature) {
		return data, nil
	}
	picture, err := decodeBitmap(data)
	if err != nil {
		return nil, err
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, picture); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}

func decodeBitmap(data []byte) (image.Image, error) {
	layout, err := bitmapLayoutOf(data)
	if err != nil {
		return nil, err
	}
	return &image.NRGBA{Pix: topDownRGBA(layout.pixels, layout.width, layout.height), Stride: layout.width * 4, Rect: image.Rect(0, 0, layout.width, layout.height)}, nil
}

type bitmapLayout struct {
	width  int
	height int
	pixels []byte
}

func bitmapLayoutOf(data []byte) (bitmapLayout, error) {
	if len(data) < bitmapHeaderSize {
		return bitmapLayout{}, ErrMalformedIcon
	}
	if binary.LittleEndian.Uint16(data[14:16]) != 32 || binary.LittleEndian.Uint32(data[16:20]) != 0 {
		return bitmapLayout{}, ErrUnsupportedIcon
	}
	headerSize := int64(binary.LittleEndian.Uint32(data[0:4]))
	width := int64(int32(binary.LittleEndian.Uint32(data[4:8])))
	height := int64(int32(binary.LittleEndian.Uint32(data[8:12]))) / 2
	if headerSize < bitmapHeaderSize || headerSize > int64(len(data)) || !isPlausibleSide(width) || !isPlausibleSide(height) {
		return bitmapLayout{}, ErrMalformedIcon
	}
	pixels := data[headerSize:]
	if int64(len(pixels)) < width*height*4 {
		return bitmapLayout{}, ErrMalformedIcon
	}
	return bitmapLayout{width: int(width), height: int(height), pixels: pixels}, nil
}

func isPlausibleSide(side int64) bool {
	return side > 0 && side <= largestIconSide
}

func topDownRGBA(bottomUpBGRA []byte, width, height int) []byte {
	rows := make([][]byte, 0, height)
	for row := range height {
		start := (height - 1 - row) * width * 4
		rows = append(rows, rgbaOf(bottomUpBGRA[start:start+width*4]))
	}
	return slices.Concat(rows...)
}

func rgbaOf(bgra []byte) []byte {
	converted := make([]byte, 0, len(bgra))
	for pixel := range slices.Chunk(bgra, 4) {
		converted = append(converted, pixel[2], pixel[1], pixel[0], pixel[3])
	}
	return converted
}

func entriesOf(ico []byte) ([]entry, error) {
	if len(ico) < 6 {
		return nil, ErrMalformedIcon
	}
	count := int(binary.LittleEndian.Uint16(ico[4:6]))
	entries := make([]entry, 0, count)
	var failures []error
	for index := range count {
		found, err := entryAt(ico, index)
		entries = append(entries, found)
		failures = append(failures, err)
	}
	return entries, errors.Join(failures...)
}

func entryAt(ico []byte, index int) (entry, error) {
	header := 6 + index*16
	if len(ico) < header+16 {
		return entry{}, ErrMalformedIcon
	}
	size := int(binary.LittleEndian.Uint32(ico[header+8:]))
	offset := int(binary.LittleEndian.Uint32(ico[header+12:]))
	if offset+size > len(ico) {
		return entry{}, ErrMalformedIcon
	}
	return entry{width: widthOf(ico[header]), data: ico[offset : offset+size]}, nil
}

func widthOf(raw byte) int {
	if raw == 0 {
		return 256
	}
	return int(raw)
}
