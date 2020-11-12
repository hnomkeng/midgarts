package spr

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"strconv"

	"github.com/pkg/errors"

	bytesutil "github.com/project-midgard/midgarts/bytes"
)

type FileType int

const (
	HeaderSignature = "SP"
	PaletteSize     = 256 * 4

	SpriteFileTypePAL FileType = iota
	SpriteFileTypeRGBA
)

type SpriteFrame struct {
	SpriteType FileType
	Width      uintptr
	Height     uintptr
	Data       []byte
}

type SpriteFile struct {
	Header struct {
		Signature string
		Version   float32

		IndexedFrameCount uint16
		RGBAFrameCount    uint16
		RGBAIndex         uint16
	}

	Frames  []*SpriteFrame
	Palette *bytes.Buffer
}

func Load(buf *bytes.Buffer) (f *SpriteFile, err error) {
	f = new(SpriteFile)

	if err := f.parseHeader(buf); err != nil {
		return nil, err
	}

	if f.Header.Version < 2.1 {
		return nil, fmt.Errorf("unsupported version %f\n", f.Header.Version)
	}

	f.parsePalette(buf)

	if err = f.readCompressedIndexedFrames(buf); err != nil {
		return nil, err
	}

	if err = f.readRGBAFrames(buf); err != nil {
		return nil, err
	}

	return f, nil
}

func (f *SpriteFile) parseHeader(buf io.Reader) error {
	var signature [2]byte
	_ = binary.Read(buf, binary.LittleEndian, &signature)

	signatureStr := string(signature[:])
	if signatureStr != HeaderSignature {
		return fmt.Errorf("invalid signature: %s\n", signature)
	}

	var major, minor byte
	_ = binary.Read(buf, binary.LittleEndian, &minor)
	_ = binary.Read(buf, binary.LittleEndian, &major)

	version, err := strconv.ParseFloat(fmt.Sprintf("%d.%d", major, minor), 32)
	if err != nil {
		return errors.Wrapf(err, "invalid version: %s\n", strconv.FormatFloat(version, 'E', -1, 32))
	}

	var indexedFrameCount, rgbaFrameCount uint16
	_ = binary.Read(buf, binary.LittleEndian, &indexedFrameCount)

	if version > 1.1 {
		_ = binary.Read(buf, binary.LittleEndian, &rgbaFrameCount)
	}

	f.Header.Signature = signatureStr
	f.Header.Version = float32(version)
	f.Header.IndexedFrameCount = indexedFrameCount
	f.Header.RGBAFrameCount = rgbaFrameCount
	f.Header.RGBAIndex = indexedFrameCount
	f.Frames = make([]*SpriteFrame, indexedFrameCount+rgbaFrameCount)
	f.Palette = bytes.NewBuffer(make([]byte, PaletteSize))

	return nil
}

// Parse .spr indexed images encoded with run-length encoding (RLE)
func (f *SpriteFile) readCompressedIndexedFrames(buf io.Reader) error {
	for i := 0; i < int(f.Header.IndexedFrameCount); i++ {
		var (
			width, height uint16
			data          []byte
		)

		_ = binary.Read(buf, binary.LittleEndian, &width)
		_ = binary.Read(buf, binary.LittleEndian, &height)

		data, err := ioutil.ReadAll(io.LimitReader(buf, int64(width*height)))
		if err != nil {
			return errors.Wrap(err, "could not read indexed frames data")
		}

		f.Frames[i] = &SpriteFrame{
			SpriteType: SpriteFileTypePAL,
			Width:      uintptr(width),
			Height:     uintptr(height),
			Data:       data,
		}
	}

	return nil
}

func (f *SpriteFile) readRGBAFrames(buf io.Reader) error {
	for i := 0; i < int(f.Header.RGBAFrameCount); i++ {
		var (
			width, height, size uint16
			data                []byte
		)

		_ = binary.Read(buf, binary.LittleEndian, &width)
		_ = binary.Read(buf, binary.LittleEndian, &height)
		size = width * height * 4

		data, err := ioutil.ReadAll(io.LimitReader(buf, int64(size)))
		if err != nil {
			return errors.Wrap(err, "could not read indexed frames data")
		}

		log.Printf("RGBA Frame: %db, \n, data=%#v\n", size, data)

		f.Frames[i+int(f.Header.RGBAIndex)] = &SpriteFrame{
			SpriteType: SpriteFileTypeRGBA,
			Width:      uintptr(width),
			Height:     uintptr(width),
			Data:       data,
		}
	}

	return nil
}

func (f *SpriteFile) parsePalette(buf *bytes.Buffer) {
	reader := bytes.NewReader(buf.Bytes())
	pos, _ := reader.Seek(0, io.SeekCurrent)
	_ = bytesutil.SkipBytes(reader, int64((reader.Len()-1024)-int(pos)))
	_, _ = io.ReadFull(io.LimitReader(reader, PaletteSize), f.Palette.Bytes())
}
