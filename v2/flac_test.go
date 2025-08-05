package flac

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

func downloadTestFile(url string) ([]byte, error) {
	zipBytes, err := httpGetBytes(url)
	if err != nil {
		return nil, err
	}
	zipfile, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, err
	}
	if zipfile.File[0].Name != "Sample_BeeMoved_96kHz24bit.flac" {
		return nil, fmt.Errorf("Unexpected test file content: %s", zipfile.File[0].Name)
	}
	flachandle, err := zipfile.File[0].Open()
	if err != nil {
		return nil, err
	}
	return io.ReadAll(flachandle)
}

func TestSelfSaveFails(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "flac")
	if err != nil {
		t.Errorf("Failed to create temporary file: %s", err)
		t.FailNow()
	}
	defer tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	flacBytes, err := downloadTestFile("http://helpguide.sony.net/high-res/sample1/v1/data/Sample_BeeMoved_96kHz24bit.flac.zip")
	if err != nil {
		t.Errorf("Error while downloading test file: %s", err.Error())
	}

	if _, err := io.Copy(tmpFile, bytes.NewReader(flacBytes)); err != nil {
		t.Errorf("Failed to write flac file: %s", err)
		t.FailNow()
	}

	f, err := ParseFile(tmpFile.Name())
	if err != nil {
		t.Errorf("Failed to parse flac file: %s", err)
	}

	filePtr := isFileBacked(f.Frames)
	if filePtr == nil {
		t.Errorf("File should be backed by a file")
		t.FailNow()
	}
	fd := filePtr.Fd()

	if file := os.NewFile(fd, "flac"); file == nil {
		t.Errorf("File should be open after calling ParseFile")
		t.FailNow()
	}

	if err := f.Save(tmpFile.Name()); err == nil {
		t.Errorf("Save should have failed")
		t.FailNow()
	}

	if err := f.Close(); err != nil {
		t.Errorf("Failed to close flac file: %s", err)
	}

	if file := os.NewFile(fd, "flac"); file != nil {
		if err := file.Close(); err == nil {
			t.Errorf("File should have been closed after calling Close")
		}
	}
}

func TestFLACDecode(t *testing.T) {
	flacBytes, err := downloadTestFile("http://helpguide.sony.net/high-res/sample1/v1/data/Sample_BeeMoved_96kHz24bit.flac.zip")
	if err != nil {
		t.Errorf("Error while downloading test file: %s", err.Error())
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

func helperGetAudioDataFromBytes(t *testing.T, flacData []byte) []byte {
	t.Helper()
	file, err := ParseBytes(bytes.NewReader(flacData))
	require.NoError(t, err, "ParseBytes should not fail when getting audio data")
	defer file.Close()

	audio, err := io.ReadAll(file.Frames)
	require.NoError(t, err, "Should be able to read all audio frames")
	return audio
}

func helperGetFileHash(t *testing.T, filePath string) []byte {
	t.Helper()
	data, err := os.ReadFile(filePath)
	require.NoError(t, err, "Failed to read file for hashing")
	hash := sha256.Sum256(data)
	return hash[:]
}

// TestInPlaceSave tests the file-shifting logic for saving over the same file using a real FLAC file.
func TestInPlaceSave(t *testing.T) {
	sourceFlacBytes, err := os.ReadFile("../testdata/data.flac")
	require.NoError(t, err, "Should be able to read the source FLAC file")

	originalAudioData := helperGetAudioDataFromBytes(t, sourceFlacBytes)
	require.NotEmpty(t, originalAudioData, "Original audio data should not be empty")

	originalFile, err := ParseBytes(bytes.NewReader(sourceFlacBytes))
	require.NoError(t, err)
	originalMetaBlockCount := len(originalFile.Meta)
	originalFile.Close()

	t.Run("Larger Metadata (Expand File)", func(t *testing.T) {
		tempFilePath := filepath.Join(t.TempDir(), "test.flac")
		err := os.WriteFile(tempFilePath, sourceFlacBytes, 0644)
		require.NoError(t, err)

		file, err := ParseFile(tempFilePath)
		require.NoError(t, err, "ParseFile of temp copy should succeed")

		file.Meta = append(file.Meta, &MetaDataBlock{})
		require.Len(t, file.Meta, originalMetaBlockCount+1, "Metadata block count should be incremented")

		err = file.Save(tempFilePath)
		require.NoError(t, err, "Save() should not fail")

		savedFileBytes, err := os.ReadFile(tempFilePath)
		require.NoError(t, err)
		savedAudioData := helperGetAudioDataFromBytes(t, savedFileBytes)
		require.Equal(t, originalAudioData, savedAudioData, "Audio data was corrupted during in-place save (expand)")

		finalFile, err := ParseFile(tempFilePath)
		require.NoError(t, err)
		require.Len(t, finalFile.Meta, originalMetaBlockCount+1, "Final file should have the new metadata block count")
		finalFile.Close()
	})

	t.Run("Smaller Metadata (Shrink File)", func(t *testing.T) {
		tempFilePath := filepath.Join(t.TempDir(), "test.flac")
		err := os.WriteFile(tempFilePath, sourceFlacBytes, 0644)
		require.NoError(t, err)

		file, err := ParseFile(tempFilePath)
		require.NoError(t, err)

		require.Greater(t, len(file.Meta), 1, "Original file must have more than 1 meta block to shrink")
		file.Meta = file.Meta[:len(file.Meta)-1]
		require.Len(t, file.Meta, originalMetaBlockCount-1)

		err = file.Save(tempFilePath)
		require.NoError(t, err, "Save() should not fail")

		savedFileBytes, err := os.ReadFile(tempFilePath)
		require.NoError(t, err)
		savedAudioData := helperGetAudioDataFromBytes(t, savedFileBytes)
		require.Equal(t, originalAudioData, savedAudioData, "Audio data was corrupted during in-place save (shrink)")

		finalFile, err := ParseFile(tempFilePath)
		require.NoError(t, err)
		require.Len(t, finalFile.Meta, originalMetaBlockCount-1)
		finalFile.Close()
	})

	t.Run("Same Size Metadata (No Change)", func(t *testing.T) {
		tempFilePath := filepath.Join(t.TempDir(), "test.flac")
		err := os.WriteFile(tempFilePath, sourceFlacBytes, 0644)
		require.NoError(t, err)

		file, err := ParseFile(tempFilePath)
		require.NoError(t, err)

		err = file.Save(tempFilePath)
		require.NoError(t, err)

		hash := sha256.Sum256(sourceFlacBytes)
		require.Equal(t, hash[:], helperGetFileHash(t, tempFilePath), "File content should be identical when saved with no changes")
	})
}
