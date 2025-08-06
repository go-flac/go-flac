package flac

import (
	"bytes"
	"errors"
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
	if fileIn := isFileBacked(c.Frames); fileIn != nil {
		fileInInfo, err := fileIn.Stat()
		if err != nil {
			return fmt.Errorf("failed to get input file info: %w", err)
		}
		fileOutInfo, err := os.Stat(fn)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to get output file info: %w", err)
		}
		if os.SameFile(fileInInfo, fileOutInfo) {
			return c.saveInPlace(fileIn, fileInInfo)
		}
	}
	f, err := os.Create(fn)
	if err != nil {
		return fmt.Errorf("failed to create FLAC output file: %w", err)
	}
	defer f.Close()

	_, err = c.WriteTo(f)
	return err
}

// saveInPlace performs a safe overwrite of the original file using a temporary file.
func (c *File) saveInPlace(originalFile *os.File, originalStat os.FileInfo) error {
	// close original so we can do rw
	if err := c.Close(); err != nil {
		return fmt.Errorf("warning: could not close original file handle: %v", err)
	}
	file, err := os.OpenFile(originalFile.Name(), os.O_RDWR, originalStat.Mode())
	if err != nil {
		return fmt.Errorf("failed to reopen file for writing: %w", err)
	}

	// i don't want bother calculating header size. so just ParseMetadata
	_, err = ParseMetadata(file)
	if err != nil {
		return fmt.Errorf("failed to parse metadata: %w", err)
	}
	originalHeaderSize, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("failed to seek to header: %w", err)
	}

	var newMetaBuf bytes.Buffer
	c.Frames = nil
	newHeaderSize, err := c.WriteTo(&newMetaBuf)
	if err != nil {
		return err
	}

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}
	totalSize := stat.Size()
	audioDataSize := totalSize - originalHeaderSize
	if audioDataSize < 0 {
		return errors.New("invalid file format: calculated audio size is negative")
	}

	delta := newHeaderSize - originalHeaderSize
	bufferSize := int64(256 * 1024) // 256KB buffer

	// New metadata is larger, shirt forward
	if delta > 0 {
		if err := file.Truncate(totalSize + delta); err != nil {
			return fmt.Errorf("failed to expand file: %w", err)
		}
		// copy data from end-to-start to avoid overwriting.
		for i := int64(0); i < audioDataSize; i += bufferSize {
			readOffset := audioDataSize - i - bufferSize
			chunkSize := bufferSize
			if readOffset < 0 {
				chunkSize += readOffset
				readOffset = 0
			}
			readPos := originalHeaderSize + readOffset
			writePos := readPos + delta
			buf := make([]byte, chunkSize)
			if _, err := file.ReadAt(buf, readPos); err != nil {
				return fmt.Errorf("read error during data shift: %w", err)
			}
			if _, err := file.WriteAt(buf, writePos); err != nil {
				return fmt.Errorf("write error during data shift: %w", err)
			}
		}
	} else if delta < 0 {
		// copy data from start-to-end.
		for i := int64(0); i < audioDataSize; i += bufferSize {
			readPos := originalHeaderSize + i
			writePos := readPos + delta
			chunkSize := bufferSize
			if readPos+chunkSize > totalSize {
				chunkSize = totalSize - readPos
			}
			if chunkSize <= 0 {
				break
			}
			buf := make([]byte, chunkSize)
			if _, err := file.ReadAt(buf, readPos); err != nil && err != io.EOF {
				return fmt.Errorf("read error during data shift: %w", err)
			}
			if _, err := file.WriteAt(buf, writePos); err != nil {
				return fmt.Errorf("write error during data shift: %w", err)
			}
		}
		if err := file.Truncate(totalSize + delta); err != nil {
			return fmt.Errorf("failed to shrink file: %w", err)
		}
	}
	// otherwise just write the header lol
	if _, err := file.WriteAt(newMetaBuf.Bytes(), 0); err != nil {
		return fmt.Errorf("failed to write new metadata: %w", err)
	}

	return file.Close()
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
