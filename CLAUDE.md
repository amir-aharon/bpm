# BPM Tempo Changer CLI

Go CLI tool that time-stretches an MP3 file to a new tempo (BPM) without changing pitch, using ffmpeg's `atempo` filter.

## Usage

```
bpm <input.mp3> <original_bpm> <target_bpm>
# e.g.: bpm song.mp3 164 170  ->  song_164bpm_to_170bpm.mp3
```

## Build & Run

```bash
go build -o bpm .
./bpm song.mp3 164 170
```

## Project Structure

```
main.go   # all logic (~110 lines, no external deps)
go.mod    # module bpm, go 1.22
```

## Key Functions

| Function | Responsibility |
|---|---|
| `main()` | arg parsing, validation, orchestration |
| `buildAtempo(ratio)` | ratio → ffmpeg filter chain string |
| `outputFilename(input, orig, target)` | construct output path |
| `formatBPM(bpm)` | float → string, no trailing zeros |
| `runFFmpeg(args...)` | exec.Command wrapper, captures stderr |

## atempo Chaining

`atempo` is clamped to `[0.5, 2.0]`. For extreme ratios, filters are chained:
- `100 -> 500` (ratio=5.0) → `atempo=2.0,atempo=2.0,atempo=1.250000`
- `100 -> 25` (ratio=0.25) → `atempo=0.5,atempo=0.500000`

## Dependencies

- **External**: `ffmpeg` must be in PATH
- **Go**: zero third-party deps
