package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>BPM Tempo Changer</title>
  <style>
    body { font-family: sans-serif; max-width: 480px; margin: 60px auto; padding: 0 1rem; }
    h1 { margin-bottom: 1.5rem; }
    label { display: block; margin-top: 1rem; font-weight: bold; }
    input[type=file], input[type=number] { width: 100%; margin-top: .25rem; padding: .4rem; box-sizing: border-box; }
    button { margin-top: 1.5rem; padding: .6rem 1.4rem; font-size: 1rem; cursor: pointer; }
  </style>
</head>
<body>
  <h1>BPM Tempo Changer</h1>
  <form method="POST" action="/process" enctype="multipart/form-data"
        onsubmit="this.querySelector('button').textContent='Processing\u2026';this.querySelector('button').disabled=true">
    <label>MP3 file
      <input type="file" name="file" accept=".mp3" required>
    </label>
    <label>Original BPM
      <input type="number" name="from" min="1" max="9999" step="any" placeholder="e.g. 164" required>
    </label>
    <label>Target BPM
      <input type="number" name="to" min="1" max="9999" step="any" placeholder="e.g. 170" required>
    </label>
    <button type="submit">Change Tempo</button>
  </form>
</body>
</html>`

type job struct {
	outputPath string
	filename   string
}

var (
	jobs   = map[string]job{}
	jobsMu sync.Mutex
)

func startServer(port string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/process", handleProcess)
	mux.HandleFunc("/download/", handleDownload)

	addr := ":" + port
	if h, err := os.Hostname(); err == nil {
		fmt.Printf("Web UI listening on http://%s%s\n", h, addr)
	}
	fmt.Printf("Web UI listening on http://0.0.0.0%s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, indexHTML)
}

func handleProcess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	origBPM, err := strconv.ParseFloat(r.FormValue("from"), 64)
	if err != nil || origBPM <= 0 {
		http.Error(w, "invalid original BPM", http.StatusBadRequest)
		return
	}
	targetBPM, err := strconv.ParseFloat(r.FormValue("to"), 64)
	if err != nil || targetBPM <= 0 {
		http.Error(w, "invalid target BPM", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".mp3") {
		http.Error(w, "only .mp3 files are supported", http.StatusBadRequest)
		return
	}

	tmpIn, err := os.CreateTemp("", "bpm-in-*.mp3")
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmpIn.Name())

	if _, err := io.Copy(tmpIn, file); err != nil {
		tmpIn.Close()
		http.Error(w, "failed to read upload", http.StatusInternalServerError)
		return
	}
	tmpIn.Close()

	suggestedName := outputFilename(header.Filename, targetBPM)
	outPath := filepath.Join(os.TempDir(), fmt.Sprintf("bpm-out-%d-%s", time.Now().UnixNano(), suggestedName))

	ratio := targetBPM / origBPM
	if err := runFFmpeg(false, "-i", tmpIn.Name(), "-filter:a", buildAtempo(ratio), "-y", outPath); err != nil {
		http.Error(w, "ffmpeg failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	jobID := fmt.Sprintf("%d", time.Now().UnixNano())
	jobsMu.Lock()
	jobs[jobID] = job{outputPath: outPath, filename: suggestedName}
	jobsMu.Unlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>BPM Tempo Changer — Done</title>
  <style>body { font-family: sans-serif; max-width: 480px; margin: 60px auto; padding: 0 1rem; }</style>
</head>
<body>
  <h1>Done!</h1>
  <p><a href="/download/%s">Download %s</a></p>
  <p><a href="/">Process another file</a></p>
</body>
</html>`, jobID, suggestedName)
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	jobID := strings.TrimPrefix(r.URL.Path, "/download/")

	jobsMu.Lock()
	j, ok := jobs[jobID]
	if ok {
		delete(jobs, jobID)
	}
	jobsMu.Unlock()

	if !ok {
		http.Error(w, "not found or already downloaded", http.StatusNotFound)
		return
	}
	defer os.Remove(j.outputPath)

	w.Header().Set("Content-Disposition", `attachment; filename="`+j.filename+`"`)
	w.Header().Set("Content-Type", "audio/mpeg")
	http.ServeFile(w, r, j.outputPath)
}
