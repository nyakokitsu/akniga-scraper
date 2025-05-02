package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"
	"flag"

	"github.com/PuerkitoBio/goquery"
	"github.com/nyakokitsu/akniga-scraper/cryptoutil"
	"github.com/nyakokitsu/akniga-scraper/downloader"
)

// Helper function to set common headers on a request
func setHeaders(req *http.Request, headers map[string]string) {
	for key, value := range headers {
		req.Header.Set(key, value)
	}
}

// parseKey extracts the LIVESTREET_SECURITY_KEY
func parseKey(content string) (string, error) {
	pattern := `LIVESTREET_SECURITY_KEY\s*=\s*'([a-fA-F0-9]+)'`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return matches[1], nil // Return the first capture group
	}
	return "", fmt.Errorf("LIVESTREET_SECURITY_KEY not found in content")
}

// Util func
func formatSecondsToHMS(totalSeconds int64) string {
	if totalSeconds < 0 {
		totalSeconds = 0
	}

	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

// Structs for parsing the final JSON response
type AjaxResponse struct {
	Title	         string `json:"titleonly"`
	TitleFull	 string `json:"title"`
	Author           string `json:"author"`
	PerformerRaw     string `json:"sTextPerformer"`	
	Poster           string `json:"preview"`
	Hres             string `json:"hres"`
	ItemsStr         string `json:"items"`
	
}

type Track struct {
	Title            string `json:"title"`
	DurationHMS      string `json:"durationhms"`
	SegmentStart     int64 `json:"time_from_start"`
	SegmentEnd       int64 `json:"time_finish"`
	//DurationHMS string `json:"durationhms"`
}

func main() {
	// --- Flags ---
	posterPtr := flag.Bool("p", false, "Download poster?")
	urlPtr := flag.String("url", "none", "Downloading url (required)")
	
	flag.Parse()
	
	fmt.Println("--- Parsed Flag Values ---")
	fmt.Println("URL:", *urlPtr)
	fmt.Println("Poster:", *posterPtr)

	pattern := `^https?://` + regexp.QuoteMeta("akniga.org") + `\/([\w-]+)-(\d+)$`
	regex, err := regexp.Compile(pattern)
	if err != nil {
		fmt.Println("Error compiling regex:", err)
		return // Cannot proceed if regex compilation fails
	}

	// --- Check for a complete match ---
	isMatch := regex.MatchString(*urlPtr)


	if *urlPtr == "none" || !isMatch {
		log.Fatalf("Failed! Incorrect url: %v", *urlPtr)
	}

	// --- Initialization ---
	initURL := "https://akniga.org/"
	bookURL := *urlPtr

	// Create a cookie jar to manage cookies
	jar, err := cookiejar.New(nil) // Use default options
	if err != nil {
		log.Fatalf("Failed to create cookie jar: %v", err)
	}

	// Create an HTTP client with the cookie jar and a timeout
	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second, // Add a reasonable timeout
	}

	// Define common headers
	commonHeaders := map[string]string{
		"User-Agent":      "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:135.0) Gecko/20100101 Firefox/135.0",
		"Accept-Language": "ru-RU,ru;q=0.8,en-US;q=0.5,en;q=0.3",
		"Connection":    "keep-alive",
		"Pragma":        "no-cache",
		"Cache-Control": "no-cache",
	}

	// --- Initial Request to establish session/cookies ---
	log.Println("🔹 Sending initial request to", initURL)
	reqInit, err := http.NewRequest("GET", initURL, nil)
	if err != nil {
		log.Fatalf("Failed to create initial request: %v", err)
	}
	// Set minimal headers for the init request
	reqInit.Header.Set("User-Agent", commonHeaders["User-Agent"])

	respInit, err := client.Do(reqInit)
	if err != nil {
		log.Fatalf("Failed to execute initial request: %v", err)
	}
	defer respInit.Body.Close() // Ensure body is closed

	if respInit.StatusCode != http.StatusOK {
		log.Printf("Initial request failed with status: %s. Cookies might not be set.", respInit.Status)
	} else {
		log.Println("Initial request successful. Cookies (if any) stored in jar.")
		// Drain the body to allow connection reuse
		_, _ = io.Copy(io.Discard, respInit.Body)
	}

	// --- Request 1: Load page content via PJAX ---
	log.Println("🔹 Sending PJAX request to", bookURL)
	reqPJAX, err := http.NewRequest("GET", bookURL, nil)
	if err != nil {
		log.Fatalf("Failed to create PJAX request: %v", err)
	}

	// Set PJAX-specific headers, merging with common ones
	pjaxHeaders := map[string]string{
		"X-Requested-With": "XMLHttpRequest",
		"X-PJAX":           "true",
		"X-PJAX-Selectors": `["main","title",".ls-block-live-wrapper",".header-title-text"]`,
		"Referer":          initURL, // Referer is the initial page
	}
	setHeaders(reqPJAX, commonHeaders) // Apply common first
	setHeaders(reqPJAX, pjaxHeaders)   // Apply/overwrite with specific

	respPJAX, err := client.Do(reqPJAX)
	if err != nil {
		log.Fatalf("Failed to execute PJAX request: %v", err)
	}
	defer respPJAX.Body.Close()

	if respPJAX.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(respPJAX.Body)
		log.Fatalf("PJAX request failed with status: %s\nResponse body: %s", respPJAX.Status, string(bodyBytes))
	}

	// Read PJAX response body
	pjaxBodyBytes, err := io.ReadAll(respPJAX.Body)
	if err != nil {
		log.Fatalf("Failed to read PJAX response body: %v", err)
	}
	pjaxBodyString := string(pjaxBodyBytes)
	// log.Printf("PJAX Page Content Snippet:\n%s\n", pjaxBodyString[:500]) // Optional: print snippet

	// --- Extract data from PJAX response ---

	// Extract security key
	securityKey, err := parseKey(pjaxBodyString)
	if err != nil {
		log.Fatalf("Failed to parse security key: %v", err)
	}
	log.Printf("🔑 Found LIVESTREET_SECURITY_KEY: %s", securityKey)

	// Extract bid using goquery
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(pjaxBodyString))
	if err != nil {
		log.Fatalf("Failed to parse PJAX HTML: %v", err)
	}

	bid, exists := doc.Find("article[data-bid]").Attr("data-bid")
	if !exists {
		log.Fatal("❌ Could not find article element with data-bid attribute")
	}
	log.Printf("🆔 Found bid: %s", bid)

	// --- Request 2: POST to get audio stream info ---
	ajaxURL := fmt.Sprintf("https://akniga.org/ajax/b/%s", bid)
	log.Println("🔹 Sending AJAX POST request to", ajaxURL)

	// Prepare form data
	postData := url.Values{}
	postData.Set("bid", bid)
	postData.Set("hls", "true")
	postData.Set("security_ls_key", securityKey)

	reqPOST, err := http.NewRequest("POST", ajaxURL, strings.NewReader(postData.Encode()))
	if err != nil {
		log.Fatalf("Failed to create POST request: %v", err)
	}

	// Set POST-specific headers
	postHeaders := map[string]string{
		"Accept":           "application/json, text/javascript, */*; q=0.01",
		"Content-Type":     "application/x-www-form-urlencoded; charset=UTF-8",
		"X-Requested-With": "XMLHttpRequest",
		"Origin":           "https://akniga.org",
		"Referer":          bookURL, // Referer is the book page
	}
	setHeaders(reqPOST, commonHeaders) // Apply common first
	setHeaders(reqPOST, postHeaders)   // Apply/overwrite with specific

	respPOST, err := client.Do(reqPOST)
	if err != nil {
		log.Fatalf("Failed to execute POST request: %v", err)
	}
	defer respPOST.Body.Close()


	if respPOST.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(respPOST.Body)
		log.Fatalf("AJAX POST request failed with status: %s\nResponse body: %s", respPOST.Status, string(bodyBytes))
	}

	// --- Parse JSON response ---
	var ajaxResp AjaxResponse
	if err := json.NewDecoder(respPOST.Body).Decode(&ajaxResp); err != nil {
		// Attempt to read body manually for debugging if JSON parsing fails
		log.Fatalf("Failed to decode initial JSON response: %v", err)
	}
	
	log.Println("✅ Got AJAX Response: ", respPOST.Status)
	log.Println("📚 Got book data!")

	// Unmarshal the inner JSON string ('items') into the Track slice
	var tracks []Track
	if err := json.Unmarshal([]byte(ajaxResp.ItemsStr), &tracks); err != nil {
		log.Fatalf("Failed to unmarshal inner 'items' JSON string: %v\nRaw 'items' string: %s", err, ajaxResp.ItemsStr)
	}

	// --- Book Data ---
	fmt.Printf("\n📚 Title: %s\n🎵 Audio duration: %v", ajaxResp.TitleFull, formatSecondsToHMS(tracks[len(tracks)-1].SegmentEnd))
	
	regPerfPattern := `<span>(.*?)</span>`
	
	regPer, err := regexp.Compile(regPerfPattern)
	if err != nil {
		fmt.Println("Error compiling regex:", err)
		return
	}
	
	matches := regPer.FindStringSubmatch(ajaxResp.PerformerRaw)
	performerName := matches[1]
	fmt.Printf("\n🔊 Performer: %s", performerName)
	fmt.Println("\n")
	
	if *posterPtr {
		if err := downloader.DownloadImage(ajaxResp.Poster, fmt.Sprintf("./books/%s/%v.png", ajaxResp.TitleFull, ajaxResp.TitleFull)); err != nil {
			log.Fatalf("Image downloading error: %v", err)
		}
	}
	
	log.Println("🔑 Trying to decode m3u8 url...")
	
	hls, err := cryptoutil.DecodeURL(ajaxResp.Hres); 
	if err != nil {
		log.Fatalf("Failed to decode. Check prev. logs.")
	}
	log.Println("✅ Got HLS! Running ffmpeg download process.")
	
	outputFile := fmt.Sprintf("./books/%s/[%s] %s.mp3", ajaxResp.TitleFull, performerName, ajaxResp.TitleFull)
	targetURL := hls
	meta := map[string]string{
		"title":  ajaxResp.Title,
		"artist": ajaxResp.Author,
		"album":  "Downloaded via akniga-scraper",
		"comment": "Downloaded via akniga-scraper",
	}

	log.Printf("Starting download for: %s", targetURL)

	err = downloader.DownloadToSingleMP3(targetURL, outputFile, meta)
	if err != nil {
		log.Fatalf("Operation failed: %v", err)
	}

	log.Printf("Download complete.")
	
}

