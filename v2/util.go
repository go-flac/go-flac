package flac

import (
	"bytes"
	"encoding/binary"
	"io"
)

type ErrorReader struct {
	err error
}

func (e *ErrorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}

type PrefixReader struct {
	prefix []byte
	r      io.Reader
}

func (c *PrefixReader) Read(p []byte) (n int, err error) {
	if len(c.prefix) == 0 {
		return c.r.Read(p)
	}
	n = copy(p, c.prefix)
	c.prefix = c.prefix[n:]
	return
}

func (c *PrefixReader) Close() error {
	if closer, ok := c.r.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func encodeUint32(n uint32) []byte {
	buf := bytes.NewBuffer([]byte{})
	if err := binary.Write(buf, binary.BigEndian, n); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func readUint8(r io.Reader) (res uint8, err error) {
	err = binary.Read(r, binary.BigEndian, &res)
	return
}

func readUint16(r io.Reader) (res uint16, err error) {
	err = binary.Read(r, binary.BigEndian, &res)
	return
}

func readUint32(r io.Reader) (res uint32, err error) {
	err = binary.Read(r, binary.BigEndian, &res)
	return
}

func checkFLACStream(f io.Reader) (io.Reader, error) {
	first2Bytes := make([]byte, 2)
	_, err := io.ReadFull(f, first2Bytes)
	if err != nil {
		return nil, err
	}

	if first2Bytes[0] != 0xFF || first2Bytes[1]>>2 != 0x3E {
		return nil, ErrorNoSyncCode
	}

	return &PrefixReader{prefix: first2Bytes, r: f}, nil
}

func parseMetadataBlock(f io.Reader) (block *MetaDataBlock, isfinal bool, err error) {
	block = new(MetaDataBlock)
	header := make([]byte, 4)
	_, err = io.ReadFull(f, header)
	if err != nil {
		return
	}
	isfinal = header[0]>>7 != 0
	block.Type = BlockType(header[0] << 1 >> 1)
	var length uint32
	err = binary.Read(bytes.NewBuffer(header), binary.BigEndian, &length)
	if err != nil {
		return
	}
	length = length << 8 >> 8

	buf := make([]byte, length)
	_, err = io.ReadFull(f, buf)
	if err != nil {
		return
	}
	block.Data = buf

	return
}

func readMetadataBlocks(f io.Reader) (blocks []*MetaDataBlock, err error) {
	finishMetaData := false
	for !finishMetaData {
		var block *MetaDataBlock
		block, finishMetaData, err = parseMetadataBlock(f)
		if err != nil {
			return
		}
		blocks = append(blocks, block)
	}
	return
}

func readFLACHead(f io.Reader) error {
	buffer := make([]byte, 4)
	_, err := io.ReadFull(f, buffer)
	if err != nil {
		return err
	}
	if string(buffer) != "fLaC" {
		return ErrorNoFLACHeader
	}
	return nil
}
