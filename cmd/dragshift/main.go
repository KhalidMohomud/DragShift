package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ZakariyaSharif/dragshift/internal/converter"
	"github.com/ZakariyaSharif/dragshift/internal/ui"
)

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "convert" {
		runCLI(os.Args[2:])
		return
	}

	if len(os.Args) >= 2 && (os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "help") {
		printUsage()
		return
	}

	fmt.Println("DragShift — Hold ⇧ while dragging a file to convert it")
	app := ui.NewApp()
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runCLI(args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: dragshift convert <file> <format>")
		fmt.Fprintln(os.Stderr, "       dragshift convert photo.heic jpg")
		fmt.Fprintln(os.Stderr, "       dragshift convert video.mov mp4")
		os.Exit(1)
	}

	src := args[0]
	ext := strings.ToLower(strings.TrimPrefix(args[1], "."))

	// Resolve the source path.
	abs, err := filepath.Abs(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	src = abs

	// Check file exists.
	if _, err := os.Stat(src); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: file not found: %s\n", src)
		os.Exit(1)
	}

	// Show what we know about this file.
	kind := converter.KindOf(src)
	if kind == "" {
		fmt.Fprintf(os.Stderr, "Error: unsupported file type: %s\n", filepath.Ext(src))
		os.Exit(1)
	}

	targets := converter.TargetsFor(src)
	if len(targets) == 0 {
		fmt.Fprintf(os.Stderr, "Error: no conversion targets for %s\n", filepath.Base(src))
		os.Exit(1)
	}

	// Check the target is valid.
	found := false
	for _, t := range targets {
		if t == ext {
			found = true
			break
		}
	}
	if !found {
		fmt.Fprintf(os.Stderr, "Error: can't convert %s to .%s\n", filepath.Ext(src), ext)
		fmt.Fprintf(os.Stderr, "Available targets: %s\n", strings.Join(targets, ", "))
		os.Exit(1)
	}

	fmt.Printf("Converting %s → .%s ...\n", filepath.Base(src), ext)

	created, err := converter.Convert(src, ext)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ Error: %v\n", err)
		os.Exit(1)
	}

	for _, c := range created {
		fmt.Printf("✓ %s\n", c)
	}
}

func printUsage() {
	fmt.Print(`DragShift — File converter for macOS

Usage:
  dragshift              Start the menu bar app (hold ⇧ while dragging)
  dragshift convert <file> <format>   Convert a file from the command line
  dragshift help         Show this help

Supported conversions:
  Images  : HEIC, PNG, JPG, WebP, GIF, TIFF, BMP → PNG, JPG, WebP, PDF, GIF
  Video   : MOV, MP4, MKV, AVI, WebM, M4V        → MP4, MOV, WebM, GIF, MP3
  Audio   : WAV, MP3, M4A, AAC, FLAC, OGG, AIFF   → MP3, M4A, WAV, FLAC, OGG
  Office  : DOCX, XLSX, ODT, RTF, TXT             → PDF
  PDF     :                                        → PNG, TXT
  Slides  : PPT, PPTX                             → PDF, PNG, JPG, PPTX, PPT

Examples:
  dragshift convert photo.heic jpg
  dragshift convert video.mov mp4
  dragshift convert document.docx pdf
  dragshift convert song.wav mp3
`)
}
