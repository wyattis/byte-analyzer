package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"

	"github.com/wyattis/z/zflag"
)

var excludePatterns = zflag.StringSlice()
var scale = 1
var includeZero = false

func isExcluded(path string) (bool, error) {
	for _, pattern := range excludePatterns.Val() {
		if matched, err := filepath.Match(pattern, path); err != nil {
			return false, err
		} else if matched {
			return true, err
		}
	}
	return false, nil
}

type ByteCount [256]int

func normalizeByteCount(byteCounts ByteCount) (normalized [256]uint8) {
	maxVal := 0
	for i, count := range byteCounts {
		if count > maxVal && (includeZero || i != 0) {
			maxVal = count
		}
	}
	if maxVal == 0 {
		return
	}
	for i := range byteCounts {
		normalized[i] = uint8(byteCounts[i] * 255 / maxVal)
	}
	return
}

func byteCountToImage(byteCounts ByteCount) (img *image.Gray) {
	size := 16
	normalized := normalizeByteCount(byteCounts)
	rect := image.Rect(0, 0, size, size)
	img = image.NewGray(rect)
	for i := 0; i < 256; i++ {
		x := i % size
		y := i / size
		img.Set(x, y, color.Gray{255 - normalized[i]})
	}
	return img
}

func addByteCounts(a, b ByteCount) (sum ByteCount) {
	for i := range a {
		sum[i] = a[i] + b[i]
	}
	return
}

func scanReader(reader io.Reader) (byteCounts ByteCount, err error) {
	buf := make([]byte, 1024)
	n := 0
	for {
		n, err = reader.Read(buf)
		if err != nil {
			if err == io.EOF {
				err = nil
				break
			}
			return
		}
		for i := 0; i < n; i++ {
			byteCounts[buf[i]]++
		}
	}
	return
}

func resizeImage(img *image.Gray, multiplier int) (resized *image.Gray) {
	width, height := img.Bounds().Dx(), img.Bounds().Dy()
	resized = image.NewGray(image.Rect(0, 0, width*multiplier, height*multiplier))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			for i := 0; i < multiplier; i++ {
				for j := 0; j < multiplier; j++ {
					resized.Set(x*multiplier+i, y*multiplier+j, img.At(x, y))
				}
			}
		}
	}
	return
}

func scanPath(path string) (byteCounts ByteCount, err error) {
	fmt.Fprintln(os.Stderr, "reading", path)
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	return scanReader(f)
}

func run() (err error) {
	flag.BoolVar(&includeZero, "include-zero", false, "include zero bytes when normalizing the output image")
	flag.IntVar(&scale, "scale", 1, "scale output image by this factor")
	flag.Var(excludePatterns, "exclude", "exclude files matching this pattern (repeated)")
	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		return
	}

	cumulative := ByteCount{}
	// fileCounts := make(map[string]ByteCount)
	for _, pattern := range flag.Args() {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return err
		}
		for _, file := range matches {
			excluded, err := isExcluded(file)
			if err != nil {
				return err
			}
			if excluded {
				continue
			}
			counts, err := scanPath(file)
			if err != nil {
				return err
			}
			// fileCounts[file] = counts
			cumulative = addByteCounts(cumulative, counts)
		}
	}
	img := byteCountToImage(cumulative)
	if scale > 1 {
		img = resizeImage(img, scale)
	}
	return png.Encode(os.Stdout, img)
}

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}
