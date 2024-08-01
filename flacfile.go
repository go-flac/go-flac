package flac

import (
	"io"
	"os"
)

// File represents a handler of FLAC file
type File struct {
	Meta   []*MetaDataBlock
	Frames io.Reader
}

// Marshal encodes all meta tags and returns the content of the resulting whole FLAC file
func (c *File) WriteTo(w io.Writer) (int64, error) {
	nInt, err := w.Write([]byte("fLaC"))
	n := int64(nInt)
	if err != nil {
		return n, err
	}
	for i, meta := range c.Meta {
		last := i == len(c.Meta)-1
		n2, err := w.Write(meta.Marshal(last))
		if err != nil {
			return n + int64(n2), err
		}
		n += int64(n2)
	}
	if c.Frames != nil {
		n2, err := io.Copy(w, c.Frames)
		if err != nil {
			return n + n2, err
		}
		n += n2
	}
	return n, nil
}

// Save encapsulates Marshal and save the file to the file system
func (c *File) Save(fn string) error {
	f, err := os.Create(fn)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = c.WriteTo(f)
	return err
}

// ParseMetadata accepts a reader to a FLAC stream and consumes only FLAC metadata
// Frames is always nil
func ParseMetadata(f io.Reader) (*File, error) {
	res := new(File)

	if err := readFLACHead(f); err != nil {
		return nil, err
	}
	meta, err := readMetadataBlocks(f)
	if err != nil {
		return nil, err
	}

	res.Meta = meta

	return res, nil
}

// ParseBytes accepts a reader to a FLAC stream and returns the final file
// FLAC audio frames are stored as a reader
func ParseBytes(f io.Reader) (*File, error) {
	res, err := ParseMetadata(f)
	if err != nil {
		return nil, err
	}

	res.Frames, err = checkFLACStream(f)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// ParseFile parses a FLAC file
// FLAC audio frames are stored as a reader
func ParseFile(filename string) (*File, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseBytes(f)
}

func (f *File) Close() error {
	if c, ok := f.Frames.(io.Closer); ok {
		return c.Close()
	}
	return nil
}
