# DragShift

Free, personal file converter for macOS. Hold **Shift** while dragging a file and a panel of formats appears next to your cursor. Drop the file on one. The converted file lands beside the original, and nothing is ever overwritten.

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
make build   # -> build/dragshift
make run     # build and start
```

A ⇧ icon appears in the menu bar. Hold Shift while dragging a file to see conversion options. You can also pick **Select File to Convert...** from the menu bar icon.

### As a macOS app (recommended)

```bash
make app     # -> build/DragShift.app (ad-hoc signed)
open build/DragShift.app
```

Running as an app bundle gives DragShift a stable identity, so macOS shows and remembers the **Automation** permission (Finder access) that the Finder-selection wheel needs. Approve the prompt on first use; you can change it later in System Settings → Privacy & Security → Automation.

Requires macOS 12 or later.

## CLI Mode

You can also convert files directly:

```bash
./dragshift convert photo.heic jpg
./dragshift convert video.mov mp4
```

## Make targets

| Target       | What it does                              |
| ------------ | ----------------------------------------- |
| `make build` | Build the binary into `build/`            |
| `make run`   | Build and run                             |
| `make app`   | Build and sign `build/DragShift.app`      |
| `make test`  | Run all tests                             |
| `make lint`  | Run `golangci-lint`                       |
| `make clean` | Remove `build/`                           |

## Project Structure

```
cmd/dragshift/      Main entry point and CLI
internal/converter/ File conversion engine
internal/ui/        macOS menu bar UI and format wheel (cgo/Objective-C)
packaging/          Info.plist for the .app bundle
```

## License

MIT
# DragShift
# devops
