# go-flac

[![Documentation](https://godoc.org/github.com/go-flac/go-flac?status.svg)](https://godoc.org/github.com/go-flac/go-flac)
[![Build Status](https://travis-ci.org/go-flac/go-flac.svg?branch=master)](https://travis-ci.org/go-flac/go-flac)

go libary for manipulating FLAC metadata

## Introduction

A FLAC(Free Lossless Audio Codec) stream generally consists of 3 parts: a "fLaC" header to mark the stream as an FLAC stream, followed by one or more metadata blocks which stores metadata and information regarding the stream, followed by one or more audio frames. This package encapsulated the operations that extract and split FLAC metadata blocks from a FLAC stream file and assembles them back after modifications provided by other packages.

## Usage

[go-flac](https://github.com/go-flac/flacpicture) provided two APIs([ParseBytes](https://godoc.org/github.com/go-flac/go-flac#ParseBytes) and [ParseFile](https://godoc.org/github.com/go-flac/go-flac#ParseFile)) to read FLAC file or byte sequence and returns a [File](https://godoc.org/github.com/go-flac/go-flac#ParseFile) struct. The [File](https://godoc.org/github.com/go-flac/go-flac#ParseFile) struct has two exported fields, Meta and Frames, the Frames consisted of raw stream data and the Meta field was a slice of all MetaDataBlocks present in the file. Other packages could parse/construct a [MetadataBlock](https://godoc.org/github.com/go-flac/go-flac#MetaDataBlock) by inspecting its Type field and apply proper decoding/encoding on the Data field of the [MetadataBlock](https://godoc.org/github.com/go-flac/go-flac#MetaDataBlock). You can modify the elements in the Meta field of a [File](https://godoc.org/github.com/go-flac/go-flac#ParseFile) as you like, as long as the StreamInfo metadata block is the first element in Meta field, according to the [specs](https://xiph.org/flac/format.html) of FLAC format.

## Examples

The following example adds a jpeg image as front cover to the FLAC metadata using [flacpicture](https://github.com/go-flac/flacpicture). 

```golang
package example

import (
    "github.com/go-flac/flacpicture"
    "github.com/go-flac/go-flac"
)

func addFLACCover(fileName string, imgData []byte) {
	f, err := flac.ParseFile(fileName)
	if err != nil {
		panic(err)
	}
	picture, err := flacpicture.NewFromImageData(flacpicture.PictureTypeFrontCover, "Front cover", imgData, "image/jpeg")
	if err != nil {
		panic(err)
	}
	picturemeta := picture.Marshal()
	f.Meta = append(f.Meta, &picturemeta)
	f.Save(fileName)
}
```