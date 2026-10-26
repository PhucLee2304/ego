package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"ego/platform/logger"
)

const (
	silenceNoise    = "-40dB"
	silenceMinSec   = "0.5"
	trailingEpsilon = 0.05
)

var (
	silenceStartExp = regexp.MustCompile(`silence_start:\s*(-?[0-9.]+)`)
	silenceEndExp   = regexp.MustCompile(`silence_end:\s*(-?[0-9.]+)`)
)

type audioInfo struct {
	DurationMs        int
	TrailingSilenceMs int
}

// probeAudioFiles đo thời lượng và khoảng lặng cuối file cho mọi audio trong manifest,
// trả map source URL → audioInfo.
func probeAudioFiles(ctx context.Context, seedDir string, manifest *mediaManifest, workers int) (map[string]audioInfo, error) {
	if workers <= 0 {
		workers = 8
	}

	entries := make([]mediaEntry, 0, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		if entry.Type == examsToeicAudioType {
			entries = append(entries, entry)
		}
	}

	result := make(map[string]audioInfo, len(entries))
	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		firstErr error
	)
	jobs := make(chan mediaEntry)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for entry := range jobs {
				info, err := probeAudio(ctx, filepath.Join(seedDir, filepath.FromSlash(entry.LocalPath)))
				mu.Lock()
				if err != nil && firstErr == nil {
					firstErr = fmt.Errorf("probe audio %s: %w", entry.LocalPath, err)
				}
				if err == nil {
					result[entry.SourceURL] = info
				}
				mu.Unlock()
			}
		}()
	}
	for _, entry := range entries {
		jobs <- entry
	}
	close(jobs)
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	logger.Log.Info().Int("audio_files", len(result)).Msg("[SEED] Probed audio files")
	return result, nil
}

func probeAudio(ctx context.Context, path string) (audioInfo, error) {
	// Ép định dạng theo đuôi file: vài mp3 có ID3 bị ffmpeg đoán nhầm thành video VVC.
	input := []string{"-i", path}
	if format := audioFormat(path); format != "" {
		input = []string{"-f", format, "-i", path}
	}

	probeArgs := append([]string{"-v", "error", "-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1"}, input...)
	out, err := exec.CommandContext(ctx, "ffprobe", probeArgs...).Output()
	if err != nil {
		return audioInfo{}, fmt.Errorf("ffprobe: %w", err)
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return audioInfo{}, fmt.Errorf("parse duration %q: %w", out, err)
	}

	var stderr bytes.Buffer
	ffmpegArgs := append([]string{"-hide_banner", "-nostats"}, input...)
	ffmpegArgs = append(ffmpegArgs, "-af", "silencedetect=noise="+silenceNoise+":d="+silenceMinSec, "-f", "null", "-")
	cmd := exec.CommandContext(ctx, "ffmpeg", ffmpegArgs...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return audioInfo{}, fmt.Errorf("ffmpeg silencedetect: %w", err)
	}

	return audioInfo{
		DurationMs:        int(duration * 1000),
		TrailingSilenceMs: int(trailingSilence(stderr.String(), duration) * 1000),
	}, nil
}

func audioFormat(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3":
		return "mp3"
	case ".wav":
		return "wav"
	default:
		return ""
	}
}

// trailingSilence lấy khoảng lặng chạm cuối file từ log silencedetect (giây).
// Khoảng lặng cuối có thể không có silence_end, hoặc silence_end ≈ duration.
func trailingSilence(log string, duration float64) float64 {
	starts := silenceStartExp.FindAllStringSubmatch(log, -1)
	if len(starts) == 0 {
		return 0
	}
	lastStart, err := strconv.ParseFloat(starts[len(starts)-1][1], 64)
	if err != nil {
		return 0
	}
	ends := silenceEndExp.FindAllStringSubmatch(log, -1)
	if len(ends) >= len(starts) {
		lastEnd, err := strconv.ParseFloat(ends[len(ends)-1][1], 64)
		if err != nil || lastEnd < duration-trailingEpsilon {
			return 0
		}
	}
	return max(0, duration-max(0, lastStart))
}
