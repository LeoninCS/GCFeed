package interfaceshttpupload

import "testing"

func TestValidateVideoMetadata(t *testing.T) {
	valid := &probeResult{
		Streams: []probeStream{
			{CodecType: "video", CodecName: "h264", Width: 1920, Height: 1080},
			{CodecType: "audio", CodecName: "aac"},
		},
		Format: probeFormat{Duration: "60.5"},
	}
	if err := validateVideoMetadata(valid); err != nil {
		t.Fatalf("expected valid metadata, got %v", err)
	}

	long := *valid
	long.Format = probeFormat{Duration: "900"}
	if err := validateVideoMetadata(&long); err == nil {
		t.Fatalf("expected long video metadata to fail")
	}

	large := *valid
	large.Streams = []probeStream{{CodecType: "video", CodecName: "h264", Width: 4096, Height: 2160}}
	if err := validateVideoMetadata(&large); err == nil {
		t.Fatalf("expected large video metadata to fail")
	}

	codec := *valid
	codec.Streams = []probeStream{{CodecType: "video", CodecName: "mpeg2video", Width: 1280, Height: 720}}
	if err := validateVideoMetadata(&codec); err == nil {
		t.Fatalf("expected unsupported codec metadata to fail")
	}
}
