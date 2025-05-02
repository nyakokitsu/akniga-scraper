package downloader

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	//"strings"
)

func DownloadToSingleMP3(m3u8URL string, outputFilename string, metadata map[string]string) error {
	if m3u8URL == "" {
		return fmt.Errorf("M3U8 URL cannot be empty")
	}
	if outputFilename == "" {
		return fmt.Errorf("output filename cannot be empty")
	}

	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg executable not found in PATH: %w", err)
	}

	outputDir := filepath.Dir(outputFilename)
	if outputDir != "." && outputDir != "/" {
		err = os.MkdirAll(outputDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create output directory for '%s': %w", outputFilename, err)
		}
	}

	fmt.Printf("books/%s/%s.png", strings.Split(outputFilename,`/`)[2], strings.Split(outputFilename,`/`)[2])
	cmdArgs := []string{
		"-i", m3u8URL,
		"-i", fmt.Sprintf("books/%s/%s.png", strings.Split(outputFilename,`/`)[2], strings.Split(outputFilename,`/`)[2]),
		"-map", "0:0",
		"-map", "1:0",
		"-id3v2_version", "3",
		"-metadata:s:v", `title="Album cover"`, 
		"-metadata:s:v", `comment="Cover (front)"`,
		"-vn",
		"-c:a", "libmp3lame",
		"-ab", "192k",
	}

	for key, value := range metadata {
		if value != "" {
			metadataArg := fmt.Sprintf("%s=\"%s\"", key, value)
			cmdArgs = append(cmdArgs, "-metadata", metadataArg)
		}
	}

	cmdArgs = append(cmdArgs, "-y", outputFilename)

	fmt.Printf("Executing FFmpeg command:\n%s %v\n", ffmpegPath, cmdArgs)

	cmd := exec.Command(ffmpegPath, cmdArgs...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("ffmpeg command failed: %v\nStderr: %s", err, stderr.String())
	}

	fmt.Printf("FFmpeg finished successfully. MP3 file written to: %s\n", outputFilename)

	return nil
}

func DownloadImage(url string, filePath string) error {
	client := http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request for %s: %w", url, err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute GET request for %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status code for %s: %d %s", url, resp.StatusCode, resp.Status)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		log.Printf("Warning: Content-Type for %s is '%s', which might not be an image.", url, contentType)
	}

	// Use the package 'filepath' and the parameter 'filePath'
	dir := filepath.Dir(filePath) // Now correctly calls the package function
	if dir != "." {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Use the renamed parameter 'filePath'
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer file.Close()

	// Use the renamed parameter 'filePath'
	bytesCopied, err := io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to copy image data to file %s: %w", filePath, err)
	}

	// Use the renamed parameter 'filePath'
	log.Printf("Successfully downloaded %s (%d bytes) to %s\n", url, bytesCopied, filePath)
	return nil
}
