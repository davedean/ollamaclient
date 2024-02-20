package ollamaclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/xyproto/env/v2"
)

const defaultPullTimeout = 48 * time.Hour // pretty generous, in case someone has a poor connection

type PullRequest struct {
	Name   string
	Stream bool
}

type PullResponse struct {
	Digest    string
	Completed int64
	Total     int64
	Status    string
}

var (
	spinner = []string{"-", "\\", "|", "/"}
	colors  = map[string]string{
		"blue":    "\033[94m",
		"cyan":    "\033[96m",
		"gray":    "\033[37m",
		"magenta": "\033[95m",
		"red":     "\033[91m",
		"white":   "\033[97m",
		"reset":   "\033[0m",
	}
)

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func generateColorizedProgressBar(progress float64, width int) string {
	progressInt := int(progress / 100 * float64(width))
	bar := colors["blue"] + strings.Repeat("=", progressInt)
	if progressInt > width/3 {
		bar += colors["magenta"] + strings.Repeat("=", max(0, progressInt-width/3))
	}
	if progressInt > 2*width/3 {
		bar += colors["cyan"] + strings.Repeat("=", max(0, progressInt-2*width/3))
	}
	bar += colors["white"] + strings.Repeat(" ", width-max(progressInt, width)) + colors["reset"]
	return bar
}

func (oc *Config) Pull(optionalVerbose ...bool) (string, error) {
	if env.Bool("NO_COLOR") {
		// Skip colors
		for k := range colors {
			colors[k] = ""
		}
	}
	verbose := oc.Verbose
	if len(optionalVerbose) > 0 && optionalVerbose[0] {
		verbose = true
	}

	reqBody := PullRequest{
		Name:   oc.Model,
		Stream: true,
	}
	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	if verbose {
		fmt.Printf("Sending request to %s/api/pull: %s\n", oc.API, string(reqBytes))
	}

	resp, err := http.Post(oc.API+"/api/pull", "application/json", bytes.NewBuffer(reqBytes))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var sb strings.Builder
	decoder := json.NewDecoder(resp.Body)

	downloadStarted := time.Now()
	spinnerPosition := 0
	var lastDigest string // Track the last hash

OUT:
	for {
		var resp PullResponse
		if err := decoder.Decode(&resp); err != nil {
			return sb.String(), err
		}

		shortDigest := strings.TrimPrefix(resp.Digest, "sha256:")
		if len(shortDigest) > 8 {
			shortDigest = shortDigest[:8]
		}

		// Check if the hash has changed (indicating a new part of the download)
		if lastDigest != "" && lastDigest != resp.Digest {
			if verbose {
				fmt.Println() // Insert a newline for a new part
			}
		}
		lastDigest = resp.Digest // Update the lastDigest for the next loop

		if resp.Total == 0 {
			if verbose {
				fmt.Printf("\r%sPulling manifest... %s%s", colors["white"], spinner[spinnerPosition%len(spinner)], colors["reset"])
				spinnerPosition++
			}
		} else {
			progress := float64(resp.Completed) / float64(resp.Total) * 100
			progressBar := generateColorizedProgressBar(progress, 30) // Fixed width bar
			displaySizeCompleted := humanize.Bytes(uint64(resp.Completed))
			displaySizeTotal := humanize.Bytes(uint64(resp.Total))

			if verbose {
				fmt.Printf("\r%s%s - %s [%s] %.2f%% - %s/%s %s", colors["white"], oc.Model, shortDigest, progressBar, progress, displaySizeCompleted, displaySizeTotal, colors["reset"])
			}
		}

		if resp.Status == "success" {
			if verbose {
				fmt.Printf("\r%s - Download complete!\033[K\n", oc.Model)
			}
			break OUT
		}

		if time.Since(downloadStarted) > defaultPullTimeout {
			return sb.String(), fmt.Errorf("downloading %s timed out after %v", oc.Model, defaultPullTimeout)
		}
	}

	return sb.String(), nil
}
