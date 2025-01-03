package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"golang.org/x/net/html"
)

// OpenAI API structures
type ImageGenerationRequest struct {
	Prompt string `json:"prompt"`
	N      int    `json:"n"`
	Size   string `json:"size"`
}

type ImageGenerationResponse struct {
	Created int `json:"created"`
	Data    []struct {
		URL string `json:"url"`
	} `json:"data"`
}

var blockedWords = []string{
	"gunshot",
	"shooting",
	"shot",
	"killed",
	"murder",
	"bombs",
	"deadly",
}

func filterHeadlines(headlines []string) []string {
	var filtered []string
	for _, headline := range headlines {
		shouldInclude := true
		headlineLower := strings.ToLower(headline)

		for _, word := range blockedWords {
			if strings.Contains(headlineLower, word) {
				shouldInclude = false
				fmt.Printf("Filtering out headline containing '%s': %s\n", word, headline)
				break
			}
		}

		if shouldInclude {
			filtered = append(filtered, headline)
		}
	}
	return filtered
}

func main() {
	// Fetch NPR headlines
	headlines, err := fetchNPRHeadlines()
	if err != nil {
		fmt.Printf("Error fetching headlines: %v\n", err)
		return
	}

	// Filter headlines
	headlines = filterHeadlines(headlines)
	if len(headlines) == 0 {
		fmt.Println("No headlines remaining after filtering")
		return
	}

	// Create output directory if it doesn't exist
	err = os.MkdirAll("generated_images", 0755)
	if err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		return
	}

	// Combine all headlines into one prompt
	combinedPrompt := "Create an artistic interpretation of today's news combining these headlines: " +
		strings.Join(headlines, ". ")

	// Truncate prompt to 1000 characters
	if len(combinedPrompt) > 1000 {
		combinedPrompt = combinedPrompt[:997] + "..."
	}

	fmt.Printf("Generating image for combined headlines...\n")
	imageURL, err := generateImage(combinedPrompt)
	if err != nil {
		fmt.Printf("Error generating image: %v\n", err)
		return
	}

	err = downloadImage(imageURL, "generated_images/combined_news.png")
	if err != nil {
		fmt.Printf("Error downloading image: %v\n", err)
		return
	}

	fmt.Println("Image generated successfully at: generated_images/combined_news.png")
}

func fetchNPRHeadlines() ([]string, error) {
	resp, err := http.Get("https://text.npr.org")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	var headlines []string
	var crawler func(*html.Node)
	crawler = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "a" {
			// Check if this is a topic-title link
			for _, attr := range node.Attr {
				if attr.Key == "class" && attr.Val == "topic-title" {
					// Get the text content of the link
					if node.FirstChild != nil {
						headlines = append(headlines, node.FirstChild.Data)
					}
					return
				}
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			crawler(c)
		}
	}
	crawler(doc)

	return headlines, nil
}

func generateImage(prompt string) (string, error) {
	// Log the prompt that will be sent to OpenAI
	fmt.Printf("\nSending prompt to OpenAI:\n%s\n\n", prompt)

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY environment variable not set")
	}

	requestBody := ImageGenerationRequest{
		Prompt: prompt,
		N:      1,
		Size:   "1024x1024",
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/images/generations", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result ImageGenerationResponse
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		fmt.Printf("API Response: %s\n", string(bodyBytes))
		return "", fmt.Errorf("failed to decode response: %v", err)
	}

	if len(result.Data) == 0 {
		fmt.Printf("API Response: %s\n", string(bodyBytes))
		return "", fmt.Errorf("no image URL in response")
	}

	return result.Data[0].URL, nil
}

func downloadImage(url, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
