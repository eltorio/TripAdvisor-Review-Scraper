package main

import (
	"encoding/csv"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/algo7/TripAdvisor-Review-Scraper/scraper/pkg/tripadvisor"
	"github.com/graphql-go/handler"
)

// var LANGUAGES = []string{"en", "fr", "pt", "es", "de", "it", "ru", "ja", "zh", "ko", "nl", "sv", "da", "fi", "no", "pl", "hu", "cs", "el", "tr", "th", "ar", "he", "id", "ms", "vi", "tl", "uk", "ro", "bg", "hr", "sr", "sk", "sl", "et", "lv", "lt", "sq", "mk", "hi", "bn", "pa", "gu", "ta", "te", "kn", "ml", "mr", "ur", "fa", "ne", "si", "my", "km", "lo", "am", "ka", "hy", "az", "uz", "tk", "ky", "tg", "mn", "bo", "sd", "ps", "ku", "gl", "eu", "ca", "is", "af", "xh", "zu", "ny", "st", "tn", "sn", "sw", "rw", "so", "mg", "eo", "cy", "gd", "gv", "ga", "mi", "sm", "to", "haw", "id", "jw"}
var LANGUAGES = []string{"en"}
var FILETYPE = "csv"

func sortReviewsByDate(reviews []tripadvisor.Review) {
	const layout = "2006-01-02"

	sort.Slice(reviews, func(i, j int) bool {
		iTime, err := time.Parse(layout, reviews[i].CreatedDate)
		if err != nil {
			log.Fatalf("Error parsing time: %v", err)
		}

		jTime, err := time.Parse(layout, reviews[j].CreatedDate)
		if err != nil {
			log.Fatalf("Error parsing time: %v", err)
		}

		return jTime.After(iTime)
	})
}

type Config struct {
	LocationURL string   `json:"LOCATION_URL"`
	Languages   []string `json:"LANGUAGES"`
	FileType    string   `json:"FILETYPE"`
	ProxyHost   string   `json:"PROXY_HOST"`
	ApiPort     string   `json:"PORT"`
}

func GetConfig() Config {
	config := Config{}
	config.LocationURL = os.Getenv("LOCATION_URL")
	if config.LocationURL == "" {
		log.Fatal("LOCATION_URL not set")
	}
	config.Languages = LANGUAGES
	if os.Getenv("LANGUAGES") != "" {
		config.Languages = strings.Split(os.Getenv("LANGUAGES"), "|")
	}
	config.FileType = FILETYPE
	if os.Getenv("FILETYPE") != "" {
		config.FileType = os.Getenv("FILETYPE")
	}
	// Get the proxy host if set
	config.ProxyHost = os.Getenv("PROXY_HOST")
	if config.FileType != "csv" && config.FileType != "json" && config.FileType != "graphql" {
		log.Fatal("Invalid file type. Use csv, json or graphql")
	}
	config.ApiPort = os.Getenv("PORT")
	if config.ApiPort == "" {
		config.ApiPort = "8080"
	}
	return config
}

func main() {
	// Scraper variables
	var allReviews []tripadvisor.Review
	var location tripadvisor.Location

	// Get the configuration
	config := GetConfig()

	log.Printf("Location URL: %s", config.LocationURL)
	log.Printf("Languages: %v", config.Languages)
	log.Printf("File Type: %s", config.FileType)

	// Get the query type from the URL
	queryType := tripadvisor.GetURLType(config.LocationURL)
	if queryType == "" {
		log.Fatal("Invalid URL")
	}
	log.Printf("Location Type: %s", queryType)

	// Parse the location ID and location name from the URL
	locationID, locationName, err := tripadvisor.ParseURL(config.LocationURL, queryType)
	if err != nil {
		log.Fatalf("Error parsing URL: %v", err)
	}
	log.Printf("Location ID: %d", locationID)
	log.Printf("Location Name: %s", locationName)

	// Get the query ID for the given query type.
	queryID := tripadvisor.GetQueryID(queryType)
	if err != nil {
		log.Fatal("The location ID must be an positive integer")
	}

	// The default HTTP client
	client := &http.Client{}

	// If the proxy host is set, use the proxy client
	if config.ProxyHost != "" {

		// Get the HTTP client with the proxy
		client, err = tripadvisor.GetHTTPClientWithProxy(config.ProxyHost)
		if err != nil {
			log.Fatalf("Error creating HTTP client with the give proxy %s: %v", config.ProxyHost, err)
		}

		// Check IP
		ip, err := tripadvisor.CheckIP(client)
		if err != nil {
			log.Fatalf("Error checking IP: %v", err)
		}
		log.Printf("IP: %s", ip)
	}

	// Fetch the review count for the given location ID
	reviewCount, err := tripadvisor.FetchReviewCount(client, locationID, queryType, config.Languages)
	if err != nil {
		log.Fatalf("Error fetching review count: %v", err)
	}
	if reviewCount == 0 {
		log.Fatalf("No reviews found for location ID %d", locationID)
	}
	log.Printf("Review count: %d", reviewCount)

	// Calculate the number of iterations required to fetch all reviews
	iterations := tripadvisor.CalculateIterations(uint32(reviewCount))
	log.Printf("Total Iterations: %d", iterations)

	// Create a slice to store the data to be written to the CSV file
	dataToWrite := make([][]string, 0, reviewCount)

	// Scrape the reviews
	for i := uint32(0); i < iterations; i++ {

		// Introduce random delay to avoid getting blocked. The delay is between 1 and 5 seconds
		delay := rand.Intn(5) + 1
		log.Printf("Iteration: %d. Delaying for %d seconds", i, delay)
		time.Sleep(time.Duration(delay) * time.Second)

		// Calculate the offset for the current iteration
		offset := tripadvisor.CalculateOffset(i)

		// Make the request to the TripAdvisor GraphQL endpoint
		resp, err := tripadvisor.MakeRequest(client, queryID, config.Languages, locationID, offset, 20)
		if err != nil {
			log.Fatalf("Error making request at iteration %d: %v", i, err)
		}

		// Check if responses is nil before dereferencing
		if resp == nil {
			log.Fatalf("Received nil response for location ID %d at iteration: %d", locationID, i)
		}

		// Now it's safe to dereference responses
		response := *resp

		// Check if the response is not empty and if the response contains reviews
		if len(response) > 0 && len(response[0].Data.Locations) > 0 {

			// Get the reviews from the response
			reviews := response[0].Data.Locations[0].ReviewListPage.Reviews

			// Append the reviews to the allReviews slice
			allReviews = append(allReviews, reviews...)

			// Store the location data
			location = response[0].Data.Locations[0].Location

			if config.FileType == "csv" {
				// Iterating over the reviews
				for _, row := range reviews {
					row := []string{
						locationName,
						row.Title,
						row.Text,
						strconv.Itoa(row.Rating),
						row.CreatedDate[0:4],
						row.CreatedDate[5:7],
						row.CreatedDate[8:10],
					}

					// Append the row to the dataToWrite slice
					dataToWrite = append(dataToWrite, row)
				}
			}

		}

	}
	if config.FileType == "csv" {
		// Create a new csv writer. We are using writeAll so defer writer.Flush() is not required
		fileName := "reviews." + config.FileType
		fileHandle, err := os.Create(fileName)
		if err != nil {
			log.Fatalf("Error creating file %s: %v", fileName, err)
		}
		defer fileHandle.Close()
		writer := csv.NewWriter(fileHandle)

		// Writing header to the CSV file
		headers := []string{"Location Name", "Title", "Text", "Rating", "Year", "Month", "Day"}
		err = writer.Write(headers)
		if err != nil {
			log.Fatalf("Error writing header to csv: %v", err)
		}
		// Write data to the CSV file
		err = writer.WriteAll(dataToWrite)
		if err != nil {
			log.Fatalf("Error writing data to csv: %v", err)
		}
	} else if config.FileType == "json" {
		// Write the data to the JSON file
		sortReviewsByDate(allReviews)

		feedback := tripadvisor.Feedback{
			Location: location,
			Reviews:  allReviews,
		}
		data, err := json.Marshal(feedback)
		if err != nil {
			log.Fatalf("Could not marshal data: %v", err)
		}
		fileName := "reviews." + config.FileType
		fileHandle, err := os.Create(fileName)
		if err != nil {
			log.Fatalf("Error creating file %s: %v", fileName, err)
		}
		defer fileHandle.Close()
		_, err = fileHandle.Write(data)
		if err != nil {
			log.Fatalf("Could not write data: %v", err)
		}
	} else if config.FileType == "graphql" {
		sortReviewsByDate(allReviews)

		feedback := tripadvisor.Feedback{
			Location: location,
			Reviews:  allReviews,
		}
		schema, err := tripadvisor.CreateSchemaFromLocalData(feedback.Reviews)
		if err != nil {
			log.Fatalf("Error creating schema: %v", err)
		}
		h := handler.New(&handler.Config{
			Schema:   &schema,
			Pretty:   true,
			GraphiQL: true,
		})

		log.Printf("Server is starting on port %s\n", config.ApiPort)
		log.Printf("Go to http://localhost:%s/graphql to use the GraphQL endpoint\n", config.ApiPort)
		http.Handle("/graphql", h)
		http.ListenAndServe(":"+config.ApiPort, nil)
	}

}

func init() {
	// Check if the environment variables are set
	if os.Getenv("LOCATION_URL") == "" {
		log.Fatal("LOCATION_URL not set")
	}
}
