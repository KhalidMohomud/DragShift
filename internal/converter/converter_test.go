package converter

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// TestKindOf
// ---------------------------------------------------------------------------

func TestKindOf(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"photo.png", "image"},
		{"photo.PNG", "image"},
		{"photo.jpg", "image"},
		{"photo.jpeg", "image"},
		{"photo.heic", "image"},
		{"photo.webp", "image"},
		{"photo.gif", "image"},
		{"photo.tiff", "image"},
		{"photo.bmp", "image"},
		{"clip.mov", "video"},
		{"clip.mp4", "video"},
		{"clip.mkv", "video"},
		{"clip.avi", "video"},
		{"clip.webm", "video"},
		{"clip.m4v", "video"},
		{"song.wav", "audio"},
		{"song.mp3", "audio"},
		{"song.m4a", "audio"},
		{"song.aac", "audio"},
		{"song.flac", "audio"},
		{"song.ogg", "audio"},
		{"song.aiff", "audio"},
		{"report.docx", "office"},
		{"data.xlsx", "office"},
		{"letter.odt", "office"},
		{"note.rtf", "office"},
		{"readme.txt", "office"},
		{"deck.pptx", "slides"},
		{"deck.ppt", "slides"},
		{"manual.pdf", "pdf"},
		{"mystery.xyz", ""},
		{"noext", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := KindOf(tt.path)
			if got != tt.want {
				t.Errorf("KindOf(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestTargetsExcludeOwnFormat
// ---------------------------------------------------------------------------

func TestTargetsExcludeOwnFormat(t *testing.T) {
	tests := []struct {
		path    string
		excluded string
	}{
		{"photo.png", "png"},
		{"photo.jpg", "jpg"},
		{"photo.jpeg", "jpg"}, // jpeg normalises to jpg
		{"clip.mp4", "mp4"},
		{"song.mp3", "mp3"},
		{"deck.pptx", "pptx"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			targets := TargetsFor(tt.path)
			for _, tgt := range targets {
				if tgt == tt.excluded {
					t.Errorf("TargetsFor(%q) includes own format %q", tt.path, tt.excluded)
				}
			}
			if len(targets) == 0 {
				t.Errorf("TargetsFor(%q) returned no targets", tt.path)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestUniquePath
// ---------------------------------------------------------------------------

func TestUniquePath(t *testing.T) {
	dir := t.TempDir()

	src := filepath.Join(dir, "photo.png")
	if err := os.WriteFile(src, []byte("png"), 0644); err != nil {
		t.Fatal(err)
	}

	// First call should return photo.jpg (does not exist).
	got := UniquePath(src, "jpg")
	want := filepath.Join(dir, "photo.jpg")
	if got != want {
		t.Errorf("UniquePath = %q, want %q", got, want)
	}

	// Create photo.jpg so next call should return photo (1).jpg.
	if err := os.WriteFile(want, []byte("jpg"), 0644); err != nil {
		t.Fatal(err)
	}
	got = UniquePath(src, "jpg")
	want = filepath.Join(dir, "photo (1).jpg")
	if got != want {
		t.Errorf("UniquePath = %q, want %q", got, want)
	}

	// Create photo (1).jpg → should get photo (2).jpg.
	if err := os.WriteFile(want, []byte("jpg"), 0644); err != nil {
		t.Fatal(err)
	}
	got = UniquePath(src, "jpg")
	want = filepath.Join(dir, "photo (2).jpg")
	if got != want {
		t.Errorf("UniquePath = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// TestImageConversion — PNG → JPG via ImageMagick
// ---------------------------------------------------------------------------

func TestImageConversion(t *testing.T) {
	if _, err := exec.LookPath("magick"); err != nil {
		if _, err := exec.LookPath("convert"); err != nil {
			t.Skip("ImageMagick not found, skipping")
		}
	}

	dir := t.TempDir()

	// Create a tiny 1×1 red PNG using ImageMagick.
	src := filepath.Join(dir, "red.png")
	cmd := exec.Command(findMagickForTest(), "xc:red", "-size", "1x1", src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create test PNG: %s: %v", out, err)
	}

	results, err := Convert(src, "jpg")
	if err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Convert returned no files")
	}
	if _, err := os.Stat(results[0]); err != nil {
		t.Errorf("output file does not exist: %v", err)
	}
	if !strings.HasSuffix(results[0], ".jpg") {
		t.Errorf("output file %q does not have .jpg extension", results[0])
	}
}

// ---------------------------------------------------------------------------
// TestNeverOverwrites
// ---------------------------------------------------------------------------

func TestNeverOverwrites(t *testing.T) {
	if _, err := exec.LookPath("magick"); err != nil {
		if _, err := exec.LookPath("convert"); err != nil {
			t.Skip("ImageMagick not found, skipping")
		}
	}

	dir := t.TempDir()
	magick := findMagickForTest()

	src := filepath.Join(dir, "pic.png")
	cmd := exec.Command(magick, "xc:blue", "-size", "1x1", src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create PNG: %s: %v", out, err)
	}

	// Pre-create pic.jpg so Convert must not overwrite it.
	existing := filepath.Join(dir, "pic.jpg")
	if err := os.WriteFile(existing, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := Convert(src, "jpg")
	if err != nil {
		t.Fatalf("Convert error: %v", err)
	}

	// The existing file must be untouched.
	data, err := os.ReadFile(existing)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original" {
		t.Error("Convert overwrote the existing file")
	}

	// The result should be a different path.
	if len(results) > 0 && results[0] == existing {
		t.Error("Convert returned the existing file path instead of a unique one")
	}
}

// ---------------------------------------------------------------------------
// TestBadTarget
// ---------------------------------------------------------------------------

func TestBadTarget(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "photo.png")
	if err := os.WriteFile(src, []byte("png"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Convert(src, "exe")
	if err == nil {
		t.Fatal("expected error for invalid target, got nil")
	}

	var ce *ConversionError
	if !isConversionError(err, &ce) {
		t.Errorf("expected *ConversionError, got %T", err)
	}
}

// ---------------------------------------------------------------------------
// TestAudioVideo — WAV → MP3 via ffmpeg
// ---------------------------------------------------------------------------

func TestAudioVideo(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found, skipping")
	}

	dir := t.TempDir()

	// Generate a 0.1-second silent WAV.
	src := filepath.Join(dir, "silence.wav")
	cmd := exec.Command("ffmpeg",
		"-f", "lavfi", "-i", "anullsrc=r=44100:cl=mono",
		"-t", "0.1", "-y", src,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create test WAV: %s: %v", out, err)
	}

	results, err := Convert(src, "mp3")
	if err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Convert returned no files")
	}
	info, err := os.Stat(results[0])
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}
	if info.Size() == 0 {
		t.Error("output file is empty")
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func findMagickForTest() string {
	if p, err := exec.LookPath("magick"); err == nil {
		return p
	}
	if p, err := exec.LookPath("convert"); err == nil {
		return p
	}
	return "magick" // will fail at exec time
}

// isConversionError is a helper that mimics errors.As for *ConversionError.
func isConversionError(err error, target **ConversionError) bool {
	if ce, ok := err.(*ConversionError); ok {
		*target = ce
		return true
	}
	return false
}
