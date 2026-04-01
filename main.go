package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	var (
		origBPMFlag = flag.Float64("from", 0, "original BPM of the input file (required)")
		targetBPMFlag = flag.Float64("to", 0, "target BPM (required)")
		outputFlag  = flag.String("o", "", "output file path (default: auto-generated next to input)")
		quiet       = flag.Bool("q", false, "suppress informational output")
		dryRun      = flag.Bool("dry-run", false, "print what would be done without running ffmpeg")
		verbose     = flag.Bool("v", false, "show ffmpeg output")
		serveFlag   = flag.Bool("serve", false, "start web UI")
		portFlag    = flag.String("port", "8080", "port for -serve mode")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: bpm -from <bpm> -to <bpm> [flags] <input.mp3>\n\n")
		fmt.Fprintf(os.Stderr, "  e.g.: bpm -from 164 -to 170 song.mp3   -> 170-song.mp3\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *serveFlag {
		startServer(*portFlag)
		return
	}

	args := flag.Args()
	if len(args) != 1 {
		flag.Usage()
		os.Exit(1)
	}
	inputFile := args[0]

	if *origBPMFlag <= 0 {
		fmt.Fprintf(os.Stderr, "Error: -from <bpm> is required and must be a positive number\n")
		os.Exit(1)
	}
	if *targetBPMFlag <= 0 {
		fmt.Fprintf(os.Stderr, "Error: -to <bpm> is required and must be a positive number\n")
		os.Exit(1)
	}
	origBPM := *origBPMFlag
	targetBPM := *targetBPMFlag

	if !strings.HasSuffix(strings.ToLower(inputFile), ".mp3") {
		fmt.Fprintf(os.Stderr, "Error: input file must have a .mp3 extension\n")
		os.Exit(1)
	}
	if !*dryRun {
		if _, err := os.Stat(inputFile); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: input file not found: %s\n", inputFile)
			os.Exit(1)
		}
		if _, err := exec.LookPath("ffmpeg"); err != nil {
			fmt.Fprintf(os.Stderr, "Error: ffmpeg not found in PATH. Please install ffmpeg.\n")
			os.Exit(1)
		}
	}

	ratio := targetBPM / origBPM
	if ratio == 1.0 {
		fmt.Fprintf(os.Stderr, "Warning: -from and -to BPM are the same — output will be identical to input.\n")
	}

	atempoChain := buildAtempo(ratio)

	outputFile := *outputFlag
	if outputFile == "" {
		outputFile = outputFilename(inputFile, targetBPM)
	}

	if !*quiet {
		fmt.Printf("Input:   %s\n", inputFile)
		fmt.Printf("Output:  %s\n", outputFile)
		fmt.Printf("Ratio:   %.6f  (%s -> %s BPM)\n", ratio, formatBPM(origBPM), formatBPM(targetBPM))
	}

	if *dryRun {
		fmt.Printf("[dry-run] ffmpeg -i %s -filter:a %q -y %s\n", inputFile, atempoChain, outputFile)
		return
	}

	if err := runFFmpeg(*verbose, "-i", inputFile, "-filter:a", atempoChain, "-y", outputFile); err != nil {
		fmt.Fprintf(os.Stderr, "Error: ffmpeg failed: %v\n", err)
		os.Exit(1)
	}

	if !*quiet {
		fmt.Printf("Done:    %s\n", outputFile)
	}
}

func buildAtempo(ratio float64) string {
	var filters []string
	for ratio < 0.5 {
		filters = append(filters, "atempo=0.5")
		ratio /= 0.5
	}
	for ratio > 2.0 {
		filters = append(filters, "atempo=2.0")
		ratio /= 2.0
	}
	filters = append(filters, fmt.Sprintf("atempo=%.6f", ratio))
	return strings.Join(filters, ",")
}

func outputFilename(input string, target float64) string {
	dir := filepath.Dir(input)
	base := filepath.Base(input)
	return filepath.Join(dir, fmt.Sprintf("%s-%s", formatBPM(target), base))
}

func formatBPM(bpm float64) string {
	return strconv.FormatFloat(bpm, 'f', -1, 64)
}

func runFFmpeg(verbose bool, args ...string) error {
	cmd := exec.Command("ffmpeg", args...)
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
