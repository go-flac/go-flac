package flac

import (
	"bytes"
	_ "embed"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed sample_header_truncated.flac
var sampleFileBytes []byte

func consumeReaderAssertEqual(t *testing.T, lh io.Reader, rh io.Reader) {
	lhBytes, err := io.ReadAll(lh)
	require.NoError(t, err)
	rhBytes, err := io.ReadAll(rh)
	require.NoError(t, err)
	require.Equal(t, lhBytes, rhBytes)
	if closer, ok := lh.(io.Closer); ok {
		closer.Close()
	}
	if closer, ok := rh.(io.Closer); ok {
		closer.Close()
	}
}

func TestFLACDecode(t *testing.T) {
	flacBytes := sampleFileBytes[:]
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

// TestFileSave tests the file saving (including in-place and out-of-place) logic for saving over the same file using a real FLAC file.
func TestFileSave(t *testing.T) {
	sourceFlacBytes := sampleFileBytes[:]

	originalAudioData := helperGetAudioDataFromBytes(t, sourceFlacBytes)
	require.NotEmpty(t, originalAudioData, "Original audio data should not be empty")

	originalFile, err := ParseBytes(bytes.NewReader(sourceFlacBytes))
	require.NoError(t, err)
	originalMetaBlockCount := len(originalFile.Meta)
	originalFile.Close()

	var pattern = make([]byte, 512<<10+2)
	for i := range pattern {
		pattern[i] = byte(i % 53)
	}

	t.Run("Roundtrip Metadata Alter and Save", func(t *testing.T) {
		paddingLenBase := 1
		for paddingLenBase < 512<<10 {
			paddingLenBase *= 2

			for paddingLenDelta := -2; paddingLenDelta <= 2; paddingLenDelta++ {
				paddingLen := paddingLenBase + paddingLenDelta

				tempFile, err := os.CreateTemp("", "*_test_input.flac")
				require.NoError(t, err)
				_, err = tempFile.Write(sourceFlacBytes)
				require.NoError(t, err)
				tempFilePath := tempFile.Name()
				tempFile.Close()

				symLinkFile := tempFilePath[:len(tempFilePath)-len("_test_input.flac")] + "_test_input.flac.link"
				err = os.Symlink(tempFilePath, symLinkFile)
				require.NoError(t, err)

				outOfPlaceFilePath := tempFilePath[:len(tempFilePath)-len("_test_input.flac")] + "_test_output.flac"

				for _, outputFilePath := range []string{tempFilePath, symLinkFile, outOfPlaceFilePath} {
					require.NoError(t, err)

					file, err := ParseFile(tempFilePath)
					require.NoError(t, err, "ParseFile of temp copy should succeed")

					file.Meta = append(file.Meta, &MetaDataBlock{
						Type: Padding,
						Data: pattern[:paddingLen],
					})
					require.Len(t, file.Meta, originalMetaBlockCount+1, "Metadata block count should be incremented")

					err = file.Save(outputFilePath) // save the file back out, this time expanding the file
					require.NoError(t, err, "Save() should not fail")

					savedFileBytes, err := os.ReadFile(outputFilePath)
					require.NoError(t, err)
					savedAudioData := helperGetAudioDataFromBytes(t, savedFileBytes)
					require.Equal(t, originalAudioData, savedAudioData, "Audio data was corrupted during in-place save (expand)")
					consumeReaderAssertEqual(t, bytes.NewReader(originalAudioData), bytes.NewReader(savedAudioData))

					finalFile, err := ParseFile(outputFilePath)
					require.NoError(t, err)
					require.Len(t, finalFile.Meta, originalMetaBlockCount+1, "Final file should have the new metadata block count")

					var lastPaddingIndex int
					for i, meta := range finalFile.Meta {
						if meta.Type == Padding {
							lastPaddingIndex = i
						}
					}
					require.Equal(t, paddingLen, len(finalFile.Meta[lastPaddingIndex].Data), "Padding length should be equal to the original padding length, paddingLen=%d", paddingLen)
					finalFile.Meta = append(finalFile.Meta[:lastPaddingIndex], finalFile.Meta[lastPaddingIndex+1:]...) // take the block back out
					err = finalFile.Save(outputFilePath)                                                               // save the file back out, this time shortening the file
					require.NoError(t, err)
					finalFile.Close()

					finalFileBytes, err := os.ReadFile(outputFilePath)
					require.NoError(t, err)
					assert.Equal(t, sourceFlacBytes, finalFileBytes, "Final file should be identical to the original")
				}
			}
		}

	})
}
