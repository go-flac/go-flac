package flac

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"testing"
)

func httpGetBytes(url string) ([]byte, error) {
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP status %d", res.StatusCode)
	}
	return io.ReadAll(res.Body)
}

func TestFLACDecode(t *testing.T) {
	zipBytes, err := httpGetBytes("http://helpguide.sony.net/high-res/sample1/v1/data/Sample_BeeMoved_96kHz24bit.flac.zip")
	if err != nil {
		t.Errorf("Error while downloading test file: %s", err.Error())
		t.FailNow()
	}
	zipfile, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Errorf("Error while decompressing test file: %s", err.Error())
		t.FailNow()
	}
	if zipfile.File[0].Name != "Sample_BeeMoved_96kHz24bit.flac" {
		t.Errorf("Unexpected test file content: %s", zipfile.File[0].Name)
		t.FailNow()
	}

	flachandle, err := zipfile.File[0].Open()
	if err != nil {
		t.Errorf("Failed to decompress test file: %s", err)
		t.FailNow()
	}

	flacBytes, err := io.ReadAll(flachandle)
	if err != nil {
		t.Errorf("Failed to read flac file: %s", err)
		t.FailNow()
	}

	verify := func(f *File) {
		metadata := [][]int{
			{0, 34},
			{4, 149},
			{6, 58388},
			{2, 1402},
			{1, 102},
		}

		for i, meta := range f.Meta {
			if BlockType(metadata[i][0]) != meta.Type {
				t.Errorf("Metadata type mismatch: got %d expected %d", meta.Type, metadata[i][0])
				t.Fail()
			}
			if metadata[i][1] != len(meta.Data) {
				t.Errorf("Metadata size mismatch: got %d expected %d", len(meta.Data), metadata[i][1])
				t.Fail()
			}
		}

		streaminfo, err := f.GetStreamInfo()
		if err != nil {
			t.Errorf("Failed to get stream info %s", err.Error())
			t.Fail()
		}
		expectedstreaminfo := &StreamInfoBlock{
			1152,
			1152,
			1650,
			6130,
			96000,
			2,
			24,
			3828096,
			[]byte{229, 209, 0, 198, 63, 81, 136, 144, 12, 102, 182, 166, 160, 140, 226, 235},
		}
		errNotEqual := func() {
			t.Error("Streaminfo does not equal.")
			t.Fail()
		}
		if !reflect.DeepEqual(*streaminfo, *expectedstreaminfo) {
			errNotEqual()
		}
	}

	f, err := ParseBytes(bytes.NewReader(flacBytes))
	if err != nil {
		t.Errorf("Failed to parse flac file: %s", err)
		t.FailNow()
	}

	verify(f)

	loopback := new(bytes.Buffer)

	if _, err := f.WriteTo(loopback); err != nil {
		t.Errorf("Failed to write flac file: %s", err)
		t.FailNow()
	}

	if !bytes.Equal(flacBytes, loopback.Bytes()) {
		t.Errorf("Loopback data does not match original")
		t.FailNow()
	}

	f, err = ParseBytes(loopback)
	if err != nil {
		t.Errorf("Failed to parse flac file: %s", err)
		t.FailNow()
	}
	verify(f)

	newLoopback := new(bytes.Buffer)

	if _, err := f.WriteTo(newLoopback); err != nil {
		t.Errorf("Failed to write flac file: %s", err)
		t.FailNow()
	}

	if !bytes.Equal(flacBytes, newLoopback.Bytes()) {
		t.Errorf("Loopback data does not match original")
		t.FailNow()
	}

	f, err = ParseMetadata(newLoopback)
	if err != nil {
		t.Errorf("Failed to parse flac file: %s", err)
		t.FailNow()
	}
	verify(f)

}
