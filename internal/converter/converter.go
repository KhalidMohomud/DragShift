// Package converter provides file format conversion using external tools
// (ImageMagick, ffmpeg, LibreOffice, poppler). It is a Go port of the
// original Python DragShift conversion engine.
package converter

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ---------------------------------------------------------------------------
// Format registry
// ---------------------------------------------------------------------------

// imageExts lists source image extensions.
var imageExts = map[string]bool{
	".heic": true, ".heif": true, ".png": true, ".jpg": true, ".jpeg": true,
	".webp": true, ".gif": true, ".tiff": true, ".tif": true, ".bmp": true,
}

// imageTargets lists valid output formats for images.
var imageTargets = []string{"png", "jpg", "webp", "pdf", "gif", "pptx"}

// videoExts lists source video extensions.
var videoExts = map[string]bool{
	".mov": true, ".mp4": true, ".mkv": true, ".avi": true,
	".webm": true, ".m4v": true,
}

// videoTargets lists valid output formats for videos.
var videoTargets = []string{"mp4", "mov", "webm", "gif", "mp3"}

// audioExts lists source audio extensions.
var audioExts = map[string]bool{
	".wav": true, ".mp3": true, ".m4a": true, ".aac": true,
	".flac": true, ".ogg": true, ".aiff": true, ".aif": true,
}

// audioTargets lists valid output formats for audio.
var audioTargets = []string{"mp3", "m4a", "wav", "flac", "ogg"}

// officeExts lists source office-document extensions.
var officeExts = map[string]bool{
	".docx": true, ".doc": true, ".xlsx": true, ".xls": true,
	".odt": true, ".rtf": true, ".txt": true,
}

// officeTargets lists valid output formats for office documents.
var officeTargets = []string{"pdf"}

// slidesExts lists source presentation extensions.
var slidesExts = map[string]bool{
	".ppt": true, ".pptx": true,
}

// slidesTargets lists valid output formats for presentations.
var slidesTargets = []string{"pdf", "png", "jpg", "pptx", "ppt"}

// pdfTargets lists valid output formats for PDF files.
var pdfTargets = []string{"png", "txt", "pptx"}

// ---------------------------------------------------------------------------
// ConversionError
// ---------------------------------------------------------------------------

// ConversionError is a custom error type for conversion failures.
type ConversionError struct {
	Src     string
	Target  string
	Message string
	Err     error
}

func (e *ConversionError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("conversion %s → %s: %s: %v", e.Src, e.Target, e.Message, e.Err)
	}
	return fmt.Sprintf("conversion %s → %s: %s", e.Src, e.Target, e.Message)
}

func (e *ConversionError) Unwrap() error { return e.Err }

// ---------------------------------------------------------------------------
// Public helpers
// ---------------------------------------------------------------------------

// KindOf returns the high-level kind for the given file path:
// "image", "video", "audio", "slides", "office", "pdf", or "" if unknown.
func KindOf(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch {
	case ext == ".pdf":
		return "pdf"
	case slidesExts[ext]:
		return "slides"
	case imageExts[ext]:
		return "image"
	case videoExts[ext]:
		return "video"
	case audioExts[ext]:
		return "audio"
	case officeExts[ext]:
		return "office"
	default:
		return ""
	}
}

// TargetsFor returns the valid conversion targets for the file at path,
// excluding the file's own format.
func TargetsFor(path string) []string {
	kind := KindOf(path)
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")

	var all []string
	switch kind {
	case "image":
		all = imageTargets
	case "video":
		all = videoTargets
	case "audio":
		all = audioTargets
	case "office":
		all = officeTargets
	case "slides":
		all = slidesTargets
	case "pdf":
		all = pdfTargets
	default:
		return nil
	}

	// Normalise common aliases so we can filter properly.
	norm := normaliseExt(ext)

	out := make([]string, 0, len(all))
	for _, t := range all {
		if normaliseExt(t) != norm {
			out = append(out, t)
		}
	}
	return out
}

// UniquePath returns an output path next to src with the given extension
// that does not already exist on disk. If "photo.png" is taken it tries
// "photo (1).png", "photo (2).png", etc.
func UniquePath(src, ext string) string {
	ext = strings.TrimPrefix(ext, ".")
	dir := filepath.Dir(src)
	base := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))

	candidate := filepath.Join(dir, base+"."+ext)
	if !fileExists(candidate) {
		return candidate
	}

	for i := 1; ; i++ {
		candidate = filepath.Join(dir, fmt.Sprintf("%s (%d).%s", base, i, ext))
		if !fileExists(candidate) {
			return candidate
		}
	}
}

// ---------------------------------------------------------------------------
// Convert
// ---------------------------------------------------------------------------

// Convert converts src to the format indicated by ext (e.g. "jpg", "mp3").
// It returns the list of created file paths, or a ConversionError.
func Convert(src, ext string) ([]string, error) {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	kind := KindOf(src)

	if kind == "" {
		return nil, &ConversionError{Src: src, Target: ext, Message: "unsupported source format"}
	}

	// Validate target.
	targets := TargetsFor(src)
	if !contains(targets, ext) {
		return nil, &ConversionError{
			Src:     src,
			Target:  ext,
			Message: fmt.Sprintf("invalid target %q for %s file", ext, kind),
		}
	}

	switch kind {
	case "image":
		return convertImage(src, ext)
	case "video":
		return convertVideo(src, ext)
	case "audio":
		return convertAudio(src, ext)
	case "office":
		return convertOffice(src, ext)
	case "slides":
		return convertSlides(src, ext)
	case "pdf":
		return convertPDF(src, ext)
	default:
		return nil, &ConversionError{Src: src, Target: ext, Message: "unsupported kind"}
	}
}

// ---------------------------------------------------------------------------
// Image conversion
// ---------------------------------------------------------------------------

func convertImage(src, ext string) ([]string, error) {
	if ext == "pptx" {
		return nil, &ConversionError{Src: src, Target: ext, Message: "PPTX generation not yet supported in Go version"}
	}

	dst := UniquePath(src, ext)
	srcExt := strings.ToLower(filepath.Ext(src))

	// For HEIC, try sips first (macOS built-in), then fall back to magick.
	if srcExt == ".heic" {
		if sipsPath, err := exec.LookPath("sips"); err == nil {
			// sips only outputs a few formats. Map ext → sips format string.
			sipsFmt := sipsFormat(ext)
			if sipsFmt != "" {
				cmd := exec.Command(sipsPath, "-s", "format", sipsFmt, src, "--out", dst)
				if out, err := cmd.CombinedOutput(); err == nil {
					return []string{dst}, nil
				} else {
					// Fall through to magick.
					_ = out
				}
			}
		}
	}

	// Try magick, then fall back to convert (older ImageMagick).
	magick := findMagick()
	if magick == "" {
		return nil, &ConversionError{Src: src, Target: ext, Message: "ImageMagick not found (install magick or convert)"}
	}

	// image → PDF: use ImageMagick.
	// Generic image conversion.
	cmd := exec.Command(magick, src, dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, &ConversionError{Src: src, Target: ext, Message: string(out), Err: err}
	}

	return []string{dst}, nil
}

// ---------------------------------------------------------------------------
// Video conversion
// ---------------------------------------------------------------------------

func convertVideo(src, ext string) ([]string, error) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, &ConversionError{Src: src, Target: ext, Message: "ffmpeg not found", Err: err}
	}

	dst := UniquePath(src, ext)

	var args []string
	switch ext {
	case "mp4":
		args = []string{"-i", src, "-c:v", "libx264", "-c:a", "aac", "-y", dst}
	case "mov":
		args = []string{"-i", src, "-c:v", "libx264", "-c:a", "aac", "-y", dst}
	case "webm":
		args = []string{"-i", src, "-c:v", "libvpx-vp9", "-c:a", "libopus", "-y", dst}
	case "gif":
		args = []string{
			"-i", src,
			"-vf", "fps=10,scale=480:-1:flags=lanczos",
			"-y", dst,
		}
	case "mp3":
		args = []string{"-i", src, "-vn", "-acodec", "libmp3lame", "-q:a", "2", "-y", dst}
	default:
		return nil, &ConversionError{Src: src, Target: ext, Message: "unsupported video target"}
	}

	cmd := exec.Command(ffmpegPath, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, &ConversionError{Src: src, Target: ext, Message: string(out), Err: err}
	}

	return []string{dst}, nil
}

// ---------------------------------------------------------------------------
// Audio conversion
// ---------------------------------------------------------------------------

func convertAudio(src, ext string) ([]string, error) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, &ConversionError{Src: src, Target: ext, Message: "ffmpeg not found", Err: err}
	}

	dst := UniquePath(src, ext)

	var args []string
	switch ext {
	case "mp3":
		args = []string{"-i", src, "-acodec", "libmp3lame", "-q:a", "2", "-y", dst}
	case "m4a":
		args = []string{"-i", src, "-c:a", "aac", "-y", dst}
	case "wav":
		args = []string{"-i", src, "-y", dst}
	case "flac":
		args = []string{"-i", src, "-y", dst}
	case "ogg":
		args = []string{"-i", src, "-c:a", "libvorbis", "-y", dst}
	default:
		return nil, &ConversionError{Src: src, Target: ext, Message: "unsupported audio target"}
	}

	cmd := exec.Command(ffmpegPath, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, &ConversionError{Src: src, Target: ext, Message: string(out), Err: err}
	}

	return []string{dst}, nil
}

// ---------------------------------------------------------------------------
// Office document conversion (via LibreOffice)
// ---------------------------------------------------------------------------

func convertOffice(src, ext string) ([]string, error) {
	sofficePath := findSoffice()
	if sofficePath == "" {
		return nil, &ConversionError{Src: src, Target: ext, Message: "LibreOffice (soffice) not found"}
	}

	dir := filepath.Dir(src)
	cmd := exec.Command(sofficePath, "--headless", "--convert-to", ext, "--outdir", dir, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, &ConversionError{Src: src, Target: ext, Message: string(out), Err: err}
	}

	// LibreOffice writes <basename>.<ext> in the output dir.
	base := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
	produced := filepath.Join(dir, base+"."+ext)

	// Rename to a unique path if the produced path already existed before conversion.
	dst := UniquePath(src, ext)
	if produced != dst {
		if err := os.Rename(produced, dst); err != nil {
			// If rename fails, the file is still at produced.
			return []string{produced}, nil
		}
	}

	return []string{dst}, nil
}

// ---------------------------------------------------------------------------
// Slides conversion (via LibreOffice)
// ---------------------------------------------------------------------------

func convertSlides(src, ext string) ([]string, error) {
	if ext == "pptx" || ext == "ppt" {
		// LibreOffice can do this with --convert-to.
		return convertOffice(src, ext)
	}

	// For pdf, png, jpg → use soffice.
	sofficePath := findSoffice()
	if sofficePath == "" {
		return nil, &ConversionError{Src: src, Target: ext, Message: "LibreOffice (soffice) not found"}
	}

	dir := filepath.Dir(src)
	cmd := exec.Command(sofficePath, "--headless", "--convert-to", ext, "--outdir", dir, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, &ConversionError{Src: src, Target: ext, Message: string(out), Err: err}
	}

	base := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
	produced := filepath.Join(dir, base+"."+ext)

	dst := UniquePath(src, ext)
	if produced != dst {
		if err := os.Rename(produced, dst); err != nil {
			return []string{produced}, nil
		}
	}

	return []string{dst}, nil
}

// ---------------------------------------------------------------------------
// PDF conversion
// ---------------------------------------------------------------------------

func convertPDF(src, ext string) ([]string, error) {
	switch ext {
	case "pptx":
		return nil, &ConversionError{Src: src, Target: ext, Message: "PPTX generation not yet supported in Go version"}

	case "png":
		return pdfToPNG(src)

	case "txt":
		return pdfToTXT(src)

	default:
		return nil, &ConversionError{Src: src, Target: ext, Message: "unsupported PDF target"}
	}
}

func pdfToPNG(src string) ([]string, error) {
	dst := UniquePath(src, "png")

	magick := findMagick()
	if magick != "" {
		cmd := exec.Command(magick, "-density", "200", src, dst)
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, &ConversionError{Src: src, Target: "png", Message: string(out), Err: err}
		}
		return []string{dst}, nil
	}

	// macOS fallback via sips (limited — only first page).
	if sipsPath, err := exec.LookPath("sips"); err == nil {
		cmd := exec.Command(sipsPath, "-s", "format", "png", src, "--out", dst)
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, &ConversionError{Src: src, Target: "png", Message: string(out), Err: err}
		}
		return []string{dst}, nil
	}

	return nil, &ConversionError{Src: src, Target: "png", Message: "ImageMagick or sips not found"}
}

func pdfToTXT(src string) ([]string, error) {
	dst := UniquePath(src, "txt")

	pdftotextPath, err := exec.LookPath("pdftotext")
	if err != nil {
		return nil, &ConversionError{
			Src:     src,
			Target:  "txt",
			Message: "pdftotext not found (install poppler: brew install poppler)",
			Err:     err,
		}
	}

	cmd := exec.Command(pdftotextPath, "-layout", src, dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, &ConversionError{Src: src, Target: "txt", Message: string(out), Err: err}
	}

	return []string{dst}, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// normaliseExt maps common aliases to a canonical form so that e.g.
// a .jpg file does not list "jpg" as a target.
func normaliseExt(e string) string {
	e = strings.ToLower(strings.TrimPrefix(e, "."))
	switch e {
	case "jpeg":
		return "jpg"
	case "tif":
		return "tiff"
	case "aif":
		return "aiff"
	case "heif":
		return "heic"
	default:
		return e
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// findMagick returns the path to ImageMagick's `magick` or legacy `convert`.
func findMagick() string {
	if p, err := exec.LookPath("magick"); err == nil {
		return p
	}
	if p, err := exec.LookPath("convert"); err == nil {
		return p
	}
	return ""
}

// findSoffice returns the path to LibreOffice's soffice binary.
func findSoffice() string {
	if p, err := exec.LookPath("soffice"); err == nil {
		return p
	}
	// Common macOS location.
	const macPath = "/Applications/LibreOffice.app/Contents/MacOS/soffice"
	if fileExists(macPath) {
		return macPath
	}
	return ""
}

// sipsFormat maps a target extension to the sips format string.
func sipsFormat(ext string) string {
	switch ext {
	case "png":
		return "png"
	case "jpg", "jpeg":
		return "jpeg"
	case "gif":
		return "gif"
	case "tiff", "tif":
		return "tiff"
	case "bmp":
		return "bmp"
	case "pdf":
		return "pdf"
	default:
		return ""
	}
}
