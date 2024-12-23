package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/spf13/cobra"
)

// SVG represents the root SVG element
type SVG struct {
	XMLName xml.Name `xml:"svg"`
	Xmlns   string   `xml:"xmlns,attr"`
	Width   string   `xml:"width,attr"`
	Height  string   `xml:"height,attr"`
	ViewBox string   `xml:"viewBox,attr"`
	Content []byte   `xml:",innerxml"`
}

// PhyloPicResponse represents the structure of the API response
type PhyloPicResponse struct {
	Links struct {
		Items []struct {
			Href  string `json:"href"`
			Title string `json:"title"`
		} `json:"items"`
	} `json:"_links"`
	Build int `json:"build"`
}

// ensureFilesDir creates the files directory if it doesn't exist
func ensureFilesDir() error {
	filesDir := "files"
	var err error
	if err = os.MkdirAll(filesDir, 0755); err != nil {
		return fmt.Errorf("failed to create files directory: %v", err)
	}
	return nil
}

// mergeSVGs combines the species SVG with the map marker template
func mergeSVGs(speciesSVGPath, outputPath string) error {
	fmt.Printf("Species SVG path: %s\n", speciesSVGPath)

	// Read species SVG
	var speciesSVGData []byte
	var err error
	speciesSVGData, err = os.ReadFile(speciesSVGPath)
	if err != nil {
		return fmt.Errorf("failed to read species SVG: %v", err)
	}

	// Parse species SVG
	var speciesSVG SVG
	err = xml.Unmarshal(speciesSVGData, &speciesSVG)
	if err != nil {
		return fmt.Errorf("failed to parse species SVG: %v", err)
	}

	// Create the combined SVG with nested species SVG
	combinedSVG := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="50" height="50">
    <!-- Map Marker -->
    <g id="map-marker" transform="translate(0, 0)">
        <path d="M14,0 C21.732,0 28,5.641 28,12.6 C28,23.963 14,36 14,36 C14,36 0,24.064 0,12.6 C0,5.641 6.268,0 14,0 Z"
            id="Shape" fill="#FF6E6E">
        </path>
    </g>
    <g id="species" transform="translate(4, 8)">
        <svg width="20" height="20" viewBox="0 0 1536 1536" preserveAspectRatio="xMidYMid meet">
            %s
        </svg>
    </g>
</svg>`, speciesSVG.Content)

	// Write the combined SVG with more restrictive permissions
	err = os.WriteFile(outputPath, []byte(combinedSVG), 0600)
	if err != nil {
		return fmt.Errorf("failed to write combined SVG: %v", err)
	}

	fmt.Printf("Created combined SVG: %s\n", outputPath)
	return nil
}

// extractUUID extracts the UUID from the image href path
func extractUUID(href string) string {
	// Remove any query parameters
	cleanPath := strings.Split(href, "?")[0]
	// Get the last part of the path which should be the UUID
	parts := strings.Split(cleanPath, "/")
	if len(parts) >= 2 && parts[1] == "images" {
		return parts[2]
	}
	return ""
}

// getVectorURL creates the vector SVG URL from the UUID
func getVectorURL(uuid string) string {
	return fmt.Sprintf("https://images.phylopic.org/images/%s/vector.svg", uuid)
}

// downloadSVG downloads the SVG file from the given URL and saves it locally
func downloadSVG(url, species, uuid string) (string, error) {
	// Create a client that follows redirects
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil // Allow redirects
		},
	}

	// Create the request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Set the Accept header for SVG
	req.Header.Set("Accept", "image/svg+xml")

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download SVG: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download SVG: status code %d", resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read SVG data: %v", err)
	}

	// Create filename from species and uuid
	filename := fmt.Sprintf("%s_%s.svg", strings.ReplaceAll(species, " ", "_"), uuid)
	filename = filepath.Join("files", filename)

	// Save the SVG file
	err = os.WriteFile(filename, body, 0600)
	if err != nil {
		return "", fmt.Errorf("failed to write SVG file: %v", err)
	}

	fmt.Printf("Downloaded SVG to: %s\n", filename)
	return filename, nil
}

// fetchPhyloPicData makes a GET request to the PhyloPic API
func fetchPhyloPicData(species string) (*PhyloPicResponse, error) {
	baseURL := "https://api.phylopic.org/images"
	params := url.Values{}
	params.Add("filter_name", species)
	params.Add("embed_items", "true")
	params.Add("embed_primaryImage", "true")
	params.Add("page", "0")

	fullURL := baseURL + "?" + params.Encode()
	fmt.Printf("Calling API URL: %s\n", fullURL)

	// Create a client that follows redirects
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil // Allow redirects
		},
	}

	// Create the initial request
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Set the Accept header
	req.Header.Set("Accept", "application/vnd.phylopic.v2+json")

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Read and parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var phyloPicResp PhyloPicResponse
	err = json.Unmarshal(body, &phyloPicResp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	// Add the build number to subsequent requests
	if phyloPicResp.Build > 0 {
		params.Set("build", fmt.Sprintf("%d", phyloPicResp.Build))
		fullURL = baseURL + "?" + params.Encode()
		fmt.Printf("Making second request with build number %d: %s\n", phyloPicResp.Build, fullURL)

		// Make a second request with the build number
		var secondReq *http.Request
		secondReq, err = http.NewRequest("GET", fullURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create second request: %v", err)
		}
		secondReq.Header.Set("Accept", "application/vnd.phylopic.v2+json")

		var secondResp *http.Response
		secondResp, err = client.Do(secondReq)
		if err != nil {
			return nil, fmt.Errorf("failed to make second request: %v", err)
		}
		defer secondResp.Body.Close()

		body, err = io.ReadAll(secondResp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read second response: %v", err)
		}

		err = json.Unmarshal(body, &phyloPicResp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse second response: %v", err)
		}
	}

	return &phyloPicResp, nil
}

// normalizeSpecies converts a species name to a normalized form:
// - converts to lowercase
// - replaces multiple spaces with single spaces
// - removes diacritics
// - trims spaces
func normalizeSpecies(s string) string {
	// Convert to lowercase
	s = strings.ToLower(s)

	// Remove diacritics
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, err := transform.String(t, s)
	if err != nil {
		return s // Return original string if transformation fails
	}

	// Replace multiple spaces with single space and trim
	return strings.Join(strings.Fields(result), " ")
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "species-map-marker",
		Short: "A CLI tool for creating map markers for species",
		Long:  `A command line interface tool that helps you create map markers for species.`,
	}

	// Make marker command
	var makeMarkerCmd = &cobra.Command{
		Use:   "make-marker [species]",
		Short: "Create a new marker",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Ensure files directory exists
			if err := ensureFilesDir(); err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			rawSpecies := args[0]
			normalizedSpecies := normalizeSpecies(rawSpecies)
			fmt.Printf("Original species name: %s\n", rawSpecies)
			fmt.Printf("Normalized species name: %s\n", normalizedSpecies)

			// Fetch data from PhyloPic API
			fmt.Printf("Fetching data from PhyloPic API for species: %s\n", normalizedSpecies)
			resp, err := fetchPhyloPicData(normalizedSpecies)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			// Print the response
			fmt.Println("\nAPI Response:")
			fmt.Println(resp)
			if len(resp.Links.Items) == 0 {
				fmt.Println("No results found for this species")
			} else {
				for _, item := range resp.Links.Items {
					uuid := extractUUID(item.Href)
					vectorURL := getVectorURL(uuid)
					fmt.Printf("Title: %s\nImage URL: %s\nUUID: %s\nVector SVG: %s\n",
						item.Title, item.Href, uuid, vectorURL)

					// Download the SVG file
					var speciesSVGPath string
					speciesSVGPath, err = downloadSVG(vectorURL, normalizedSpecies, uuid)
					if err != nil {
						fmt.Printf("Error downloading SVG: %v\n", err)
						continue
					}

					// Create combined marker
					outputPath := filepath.Join("files", fmt.Sprintf("%s_marker.svg", strings.ReplaceAll(normalizedSpecies, " ", "_")))
					err = mergeSVGs(speciesSVGPath, outputPath)
					if err != nil {
						fmt.Printf("Error creating marker: %v\n", err)
					}

					fmt.Println()
				}
			}

			fmt.Println("Marker created successfully!")
		},
	}

	// Add commands to root
	rootCmd.AddCommand(makeMarkerCmd)

	// Execute root command
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
