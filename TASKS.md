# DragShift Go rewrite: tasks

Goal: the Go version matches everything the Python original
(github.com/ZakariyaSharif/dragshift) does, in both the UI and the conversions.

Current state: `go build`, `go vet` and `go test` pass. The radial wheel, the
global Shift+drag monitor, the menu bar icon, the CLI and most conversions are
ported. The tasks below are what is still missing or behaves differently.

## P0: bugs that break the original's behaviour

- [ ] **1. Run conversions off the main thread.** `goPerformConversion` runs synchronously from the event
      monitor and from `performDragOperation`, both of which are on the main thread. A long video conversion
      freezes the wheel and the menu bar. Python ran each conversion in a background thread.
      Fix: start a goroutine inside `goPerformConversion` (internal/ui/monitor_darwin.go).
- [ ] **2. Stop one drop from converting twice.** The mouse-up handler and `performDragOperation` can both fire for the
      same drop, which produces `name.png` and `name (1).png`. Fix: add a "handled" flag per wheel session.
- [ ] **3. Office and slide conversions must never overwrite a file.** `convertOffice` and `convertSlides` run
      `soffice --outdir <source dir>`, which overwrites an existing `name.pdf`. Python converted into a temp
      dir and then moved the result to `UniquePath`. Do the same with `os.MkdirTemp`.
- [ ] **4. PDF to PNG must make one file per page.** With ImageMagick, a multi-page PDF writes `name-0.png`, `name-1.png`…
      but the code returns `name.png`, which does not exist. With `sips`, only page 1 is converted.
      Python output: `name-p1.png`, `name-p2.png`… at 200 dpi (just `name.png` for a single page).
- [ ] **5. Slides to PNG/JPG must make one file per slide.** `soffice --convert-to png` exports only the first slide.
      Python converted the slides to a PDF in a temp dir, rendered each page at 150 dpi, and named the files `name-p1.png`, and so on.
- [ ] **31. Fix "Select File to Convert..." showing nothing.** `openFileDialogAndConvert` (internal/ui/wheel_darwin.m) runs
      `NSAppleScript` with `choose file` on a background queue. NSAppleScript must run on the main thread, and it runs
      inside DragShift's own process, which is a menu bar app that never comes to the front. So the dialog either fails or
      opens hidden behind other windows. The error is thrown away (`err` is never printed), so you see nothing.
      Not in Python: this menu item is new in the Go version.
      Fix: run the two dialogs from Go with `exec.Command("osascript", "-e", script)`. That is a separate process, so the
      dialog opens in front. The script should start with `activate`. Print the error output when the user didn't cancel.
      Exit code 1 with "User canceled" means the user cancelled. Then call `converter.Convert`.

## P1: missing features

- [ ] **6. Image to PPTX** (one full-bleed slide, slide shape matches the image). Add `pptx` to `imageTargets`.
- [ ] **7. PDF to PPTX** (one slide per page, rendered at 150 dpi). Add `pptx` to `pdfTargets`.
      Go has no python-pptx. Write a small `internal/pptx` package that zips a minimal OOXML template with
      one `<p:pic>` per slide (12192000 EMU wide, height from the first image's aspect ratio, each image centred).
- [ ] **8. Choose a PDF backend.** Recommended: `github.com/gen2brain/go-fitz` (MuPDF, the same engine as PyMuPDF).
      It handles page rendering (tasks 4, 5, 7) and text extraction, so `poppler`/`pdftotext` and
      ImageMagick+Ghostscript are no longer needed.
- [ ] **9. Fix image conversion gaps.** None of the current backends handles HEIC to WebP: `sips` can't write WebP, `magick` is
      often not installed, and ffmpeg's HEIC reading is unreliable. Fix: when the target is webp, convert with `sips` to a temp PNG
      first, then run ffmpeg. Flatten alpha for JPG/PDF output, as Python did.
- [ ] **10. Menu bar feedback.** After a conversion, flash ✓ or ✗ in the menu bar title for 3 s and play the "Glass" sound
      (`NSSound`), as Python did. Right now only the dead code in `app.go` flashes; the live path only posts an `osascript` notification.

## P2: make the ffmpeg settings match Python

- [ ] **11.** mp4: add `-pix_fmt yuv420p -movflags +faststart`. mov: add `-pix_fmt yuv420p`. Without yuv420p, some
      sources produce files QuickTime won't play.
- [ ] **12.** gif: `fps=12,scale=720:-1:flags=lanczos -loop 0` (Go has fps=10, scale 480 and no loop flag).
- [ ] **13.** Audio: m4a `-b:a 192k`, flac `-c:a flac`, ogg `-q:a 5`, add `-vn` to every audio target. Add `-loglevel error`.
- [ ] **14.** Error text: show only the last stderr line, not the whole ffmpeg/soffice output, so notifications are readable.

## P2: robustness

- [ ] **15.** `Convert` should reject a source that is not a regular file ("Not a file"), as Python did. Right now only the CLI checks this.
- [ ] **16.** Add a 1-hour timeout to every external command (`exec.CommandContext`), as Python did.
- [ ] **17.** Delete partial output files on failure for every converter, not only images.
- [ ] **18.** Shorten `ConversionError.Error()` so it shows the file name, not the full path, plus a short reason.
- [ ] **19.** Remove the `generalPasteboard` fallback in `wheel_darwin.m`. It shows the wheel for files copied earlier
      whenever you Shift+drag anything, such as a window or selected text.
- [ ] **20.** Make the Finder-selection AppleScript fallback asynchronous, or remove it. It runs on the main thread on every
      Shift+drag event and triggers an Automation permission prompt.
- [ ] **21.** Ignore a stale drag pasteboard: note `changeCount` on mouse-down and show the wheel only if it has changed.
- [ ] **22.** Multi-monitor: position the wheel using the screen under the cursor, not `mainScreen` with origin 0.
- [ ] **23.** Decide whether releasing Shift mid-drag hides the wheel (Go does now). In Python, once the wheel was open it stayed open.

## P3: cleanup, tests, packaging

- [ ] **24.** Remove the dead menu-based flow in `internal/ui/app.go` (`onFilesDetected`, `waitForFormatClick`,
      `runConversion`, `formatsForExtension`). Nothing calls it.
- [ ] **25.** Port the missing Python tests: PNG with alpha to JPG, PNG to PDF to PNG to TXT round trip, WAV to MP3,
      video to GIF, video with no audio to MP3 (must fail), TXT to PDF (skip if no soffice), image/PDF to PPTX slide count,
      exact slide target list `["pdf","png","jpg","ppt"]`.
- [ ] **26.** Make the image tests run without ImageMagick: generate the fixtures with Go's `image/png`. Right now they are skipped.
      Use `errors.As` instead of the custom `isConversionError` helper.
- [ ] **27.** Move the wedge hit-test math into Go (or mirror it there) and unit-test it.
- [ ] **28.** Package a `.app` bundle (`make app`): an Info.plist with `LSUIElement=true` and an ad-hoc codesign. Permissions and
      notifications then belong to DragShift, not to Terminal. Document how to add it as a login item.
- [ ] **29.** Update the README with the final dependency list (once task 8 is done) and remove the stray `# DragShift` at the end.
- [ ] **30.** Write debug output to a `debug.log` file, as Python did, so problems can be diagnosed when the app is launched as a `.app`.
