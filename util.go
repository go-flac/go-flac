package flac

import (
	"bytes"
	"encoding/binary"
	"io"
)

func encodeUint32(n uint32) []byte {
	buf := bytes.NewBuffer([]byte{})
	if err := binary.Write(buf, binary.BigEndian, n); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func readFLACStream(f io.Reader) ([]byte, error) {
	buffer := make([]byte, 1024*1024) // read in 1M chunk
	res := bytes.NewBuffer([]byte{})
	for {
		nn, err := f.Read(buffer)
		res.Write(buffer[:nn])
		if err != nil {
			if err == io.EOF {
				result := res.Bytes()
				if result[0] != 0xFF || result[1]>>2 != 0x3E {
					return nil, ErrorNoSyncCode
				}
				return result, nil
			}
			panic(err)
		}
	}
}

func parseMetadataBlock(f io.Reader) (block *MetaDataBlock, isfinal bool, err error) {
	block = new(MetaDataBlock)
	header := make([]byte, 4)
	_, err = f.Read(header)
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

	data := bytes.NewBuffer([]byte{})

	var nn int
	for length > 0 {
		buf := make([]byte, length)
		nn, err = f.Read(buf)
		if err != nil {
			return
		}
		data.Write(buf[:nn])
		length -= uint32(nn)
	}
	block.Data = data.Bytes()

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
	_, err := f.Read(buffer)
	if err != nil {
		panic(err)
	}
	if string(buffer) != "fLaC" {
		return ErrorNoFLACHeader
	}
	return nil
}
