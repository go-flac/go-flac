package flac

import (
	"archive/zip"
	"bytes"
	"testing"

	httpclient "github.com/ddliu/go-httpclient"
	"github.com/google/go-cmp/cmp"
)

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i, _ := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestFLACDecode(t *testing.T) {
	zipres, err := httpclient.Begin().Get("http://helpguide.sony.net/high-res/sample1/v1/data/Sample_BeeMoved_96kHz24bit.flac.zip")
	if err != nil {
		t.Errorf("Error while downloading test file: %s", err.Error())
		t.FailNow()
	}
	zipdata, err := zipres.ReadAll()
	if err != nil {
		t.Errorf("Error while downloading test file: %s", err.Error())
		t.FailNow()
	}
	zipfile, err := zip.NewReader(bytes.NewReader(zipdata), int64(len(zipdata)))
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

	verify := func(f *File) {
		metadata := [][]int{
			[]int{0, 34},
			[]int{4, 149},
			[]int{6, 58388},
			[]int{2, 1402},
			[]int{1, 102},
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
		if !cmp.Equal(*streaminfo, *expectedstreaminfo) {
			errNotEqual()
		}
	}

	f, err := ParseBytes(flachandle)
	if err != nil {
		t.Errorf("Failed to parse flac file: %s", err)
		t.Fail()
	}

	verify(f)

	f, err = ParseBytes(bytes.NewReader(f.Marshal()))
	if err != nil {
		t.Errorf("Failed to parse flac file: %s", err)
		t.Fail()
	}
	verify(f)

}
