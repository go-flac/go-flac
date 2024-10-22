package flac

import (
	"fmt"
	"io"
	"os"
)

// File represents a handler of FLAC file
type File struct {
	Meta   []*MetaDataBlock
	Frames io.Reader
}

// Marshal encodes all meta tags and returns the content of the resulting whole FLAC file
// If Frames is not nil, it will be written to the output, and then the File will be closed, further calls to WriteTo will return ErrorAlreadyWritten
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
		defer func() {
			c.Frames = &ErrorReader{err: ErrorAlreadyWritten}
		}()
		defer c.Close()
		n2, err := io.Copy(w, c.Frames)
		if err != nil {
			return n + n2, err
		}
		n += n2
	}
	return n, nil
}

// Save encapsulates WriteTo by writing the edited metadata to the given path and then piping the audio stream to the output file.
// The output must not feed back into the input as the data will be corrupted when piping the audio stream.
// This is commonly caused by attempting to save the file to the same location as the input file.
// The only information this library have is an io.Reader so it is impossible to reliably detect such cases.
// Thus caller should implement logic to prevent such cases.
func (c *File) Save(fn string) error {
	f, err := os.Create(fn)
	if err != nil {
		return fmt.Errorf("failed to create FLAC output file: %w", err)
	}
	defer f.Close()

	if fileIn := isFileBacked(c.Frames); fileIn != nil {
		fileInInfo, err := fileIn.Stat()
		if err != nil {
			return fmt.Errorf("failed to get input file info: %w", err)
		}
		fileOutInfo, err := f.Stat()
		if err != nil {
			return fmt.Errorf("failed to get output file info: %w", err)
		}
		if os.SameFile(fileInInfo, fileOutInfo) {
			return fmt.Errorf("output file must not be the same as the input file")
		}
	}

	_, err = c.WriteTo(f)
	return err
}

// ParseMetadata accepts a reader to a FLAC stream and consumes only FLAC metadata
// Frames are not read
// Further calls to WriteTo will only write the metadata
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
// You should call Close() on the returned File to free resources
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
// You should call Close() on the returned File to free resources
func ParseFile(filename string) (*File, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	return ParseBytes(NewBufIOWithInner(f))
}

// Close closes the file
// If the file is already closed, it returns nil
func (f *File) Close() error {
	if c, ok := f.Frames.(io.Closer); ok {
		return c.Close()
	}
	return nil
}
