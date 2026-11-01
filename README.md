# DragShift

Free, personal file converter for macOS. Hold **Shift** while dragging a file and a panel of formats appears next to your cursor. Drop the file on one. The converted file lands beside the original, and nothing is ever overwritten.

Rewritten in Go for better performance and easier distribution.

## Supports

- Images: HEIC, PNG, JPG, WebP, GIF, TIFF, BMP → PNG, JPG, WebP, PDF, GIF
- Video: MOV, MP4, MKV, AVI, WebM, M4V → MP4, MOV, WebM, GIF, MP3
- Audio: WAV, MP3, M4A, AAC, FLAC, OGG, AIFF → MP3, M4A, WAV, FLAC, OGG
- Documents: DOCX, XLSX, ODT, RTF, TXT → PDF
- PDF → PNG (one per page), TXT
- Slides: PPT and PPTX → PDF, PNG, JPG, or to each other

## Prerequisites

```bash
brew install ffmpeg imagemagick poppler
brew install --cask libreoffice   # only needed for Office/Slides → PDF
```

## Build & Run

```bash
go build -o dragshift ./cmd/dragshift
./dragshift
```

A ⇧ icon appears in the menu bar. Hold Shift while dragging a file to see conversion options.

## CLI Mode

You can also convert files directly:

```bash
./dragshift convert photo.heic jpg
./dragshift convert video.mov mp4
```

## Test

```bash
go test ./...
```

## Project Structure

```
cmd/dragshift/     Main entry point
internal/converter/ File conversion engine
internal/ui/       macOS menu bar UI
```

## License

MIT
# DragShift
# devops
