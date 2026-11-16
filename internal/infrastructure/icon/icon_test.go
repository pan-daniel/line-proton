package icon_test

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/pan-daniel/line-proton/internal/infrastructure/icon"
	"github.com/tc-hib/winres"
)

var lineGreen = color.NRGBA{R: 6, G: 199, B: 85, A: 255}

func plainExecutable(test *testing.T) string {
	test.Helper()
	plain := filepath.Join(test.TempDir(), "plain.exe")
	build := exec.CommandContext(test.Context(), "go", "build", "-o", plain, "./testdata/hello")
	build.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
	if combined, err := build.CombinedOutput(); err != nil {
		test.Fatalf("build fixture: %v\n%s", err, combined)
	}
	return plain
}

func executableWith(test *testing.T, iconResource *winres.Icon) string {
	test.Helper()
	resources := &winres.ResourceSet{}
	if err := resources.SetIcon(winres.ID(101), iconResource); err != nil {
		test.Fatal(err)
	}
	withIcon := filepath.Join(test.TempDir(), "with-icon.exe")
	source, err := os.Open(plainExecutable(test))
	if err != nil {
		test.Fatal(err)
	}
	defer source.Close()
	target, err := os.Create(withIcon)
	if err != nil {
		test.Fatal(err)
	}
	defer target.Close()
	if err := resources.WriteToEXE(target, source); err != nil {
		test.Fatal(err)
	}
	return withIcon
}

func square(size int) image.Image {
	canvas := image.NewNRGBA(image.Rect(0, 0, size, size))
	canvas.Set(0, 0, lineGreen)
	return canvas
}

func pngIcon(test *testing.T, sizes ...int) *winres.Icon {
	test.Helper()
	images := []image.Image{}
	for _, size := range sizes {
		images = append(images, square(size))
	}
	iconResource, err := winres.NewIconFromImages(images)
	if err != nil {
		test.Fatal(err)
	}
	return iconResource
}

func bitmapIcon(test *testing.T, size int) *winres.Icon {
	test.Helper()
	header := make([]byte, 40)
	binary.LittleEndian.PutUint32(header[0:], 40)
	binary.LittleEndian.PutUint32(header[4:], uint32(size))
	binary.LittleEndian.PutUint32(header[8:], uint32(size*2))
	binary.LittleEndian.PutUint16(header[12:], 1)
	binary.LittleEndian.PutUint16(header[14:], 32)
	pixels := make([]byte, size*size*4)
	topLeft := (size - 1) * size * 4
	copy(pixels[topLeft:], []byte{lineGreen.B, lineGreen.G, lineGreen.R, lineGreen.A})
	mask := make([]byte, size*size/8)
	bitmap := bytes.Join([][]byte{header, pixels, mask}, nil)
	directory := make([]byte, 22)
	binary.LittleEndian.PutUint16(directory[2:], 1)
	binary.LittleEndian.PutUint16(directory[4:], 1)
	directory[6], directory[7] = byte(size), byte(size)
	binary.LittleEndian.PutUint16(directory[10:], 1)
	binary.LittleEndian.PutUint16(directory[12:], 32)
	binary.LittleEndian.PutUint32(directory[14:], uint32(len(bitmap)))
	binary.LittleEndian.PutUint32(directory[18:], 22)
	iconResource, err := winres.LoadICO(bytes.NewReader(append(directory, bitmap...)))
	if err != nil {
		test.Fatal(err)
	}
	return iconResource
}

func extracted(test *testing.T, executable string) image.Image {
	test.Helper()
	destination := filepath.Join(test.TempDir(), "icons", "line.png")
	if err := icon.ExtractPNG(executable, destination); err != nil {
		test.Fatal(err)
	}
	file, err := os.Open(destination)
	if err != nil {
		test.Fatal(err)
	}
	defer file.Close()
	decoded, err := png.Decode(file)
	if err != nil {
		test.Fatal(err)
	}
	return decoded
}

func TestExtractPNGWritesTheLargestIcon(test *testing.T) {
	if width := extracted(test, executableWith(test, pngIcon(test, 32, 256))).Bounds().Dx(); width != 256 {
		test.Errorf("width = %d, want 256", width)
	}
}

func TestExtractPNGConvertsBitmapIcons(test *testing.T) {
	decoded := extracted(test, executableWith(test, bitmapIcon(test, 64)))
	if decoded.Bounds().Dx() != 64 || color.NRGBAModel.Convert(decoded.At(0, 0)) != lineGreen {
		test.Errorf("bounds = %v, top-left = %v", decoded.Bounds(), decoded.At(0, 0))
	}
}
