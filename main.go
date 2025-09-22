package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/jessevdk/go-flags"
)

var opts struct {
	Website        string  `short:"w" long:"website" description:"Website URL for famly app" default:"https://app.nfamilyclub.com/" env:"WEBSITE" required:"true"`
	ChildID        string  `long:"childid" description:"Child ID for child in famly app" env:"CHILDID" required:"true"`
	Latitude       float64 `long:"latitude" description:"Latitude to use for EXIF data" env:"LATITUDE"`
	Longitude      float64 `long:"longitude" description:"Longitude to use for EXIF data" env:"LONGITUDE"`
	AccessToken    string  `long:"accessToken" description:"Access Token (x-famly-accesstoken request header)" env:"ACCESS_TOKEN" required:"true"`
	InstallationId string  `long:"installationId" description:"Installation ID (x-famly-installationid request header)" env:"INSTALLATION_ID" required:"true"`
}

func main() {
	_, err := flags.Parse(&opts)
	if err != nil {
		fmt.Println("Error parsing flags:", err)
		return
	}

	downloadedIDs, err := loadState()
	if err != nil {
		fmt.Println("Error loading state:", err)
		return
	}
	defer saveState(downloadedIDs)

	var accessToken = opts.AccessToken
	var installationId = opts.InstallationId

	// Download all images using pagination with olderThan
	var (
		oldest time.Time

		page   = 1
		client = &http.Client{}
	)
	for {
		// Build request
		req, err := http.NewRequest("GET", opts.Website+"api/v2/images/tagged", nil)
		if err != nil {
			fmt.Println("Error creating request:", err)
			return
		}
		q := req.URL.Query()
		q.Set("childId", opts.ChildID)
		q.Set("limit", "100")
		if !oldest.IsZero() {
			// Truncate the time to the start of the day as there seems to be photos with the exact
			// same created time. I'm guessing internally famly use the upload time as createdAt.
			// It's also not a big deal as we don't redownload images that already exist.
			q.Set("olderThan", oldest.Truncate(24*time.Hour).Format("2006-01-02T15:04:05-07:00"))
		}
		req.URL.RawQuery = q.Encode()
		req.Header.Set("x-famly-accesstoken", accessToken)
		req.Header.Set("x-famly-installationid", installationId)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("accept", "application/json")
		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36")

		resp, err := client.Do(req)
		if err != nil {
			fmt.Println("Error making GET request:", err)
			return
		}

		if resp.StatusCode != http.StatusOK {
			fmt.Printf("Failed to fetch images: %s\n", resp.Status)
			resp.Body.Close()
			return
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			fmt.Println("Error reading response body:", err)
			return
		}

		var taggedImages TaggedImages
		err = json.Unmarshal(body, &taggedImages)
		if err != nil {
			fmt.Println("Error unmarshalling JSON response:", err)
			return
		}
		if len(taggedImages) == 0 {
			fmt.Println("No more images to download.")
			break
		}
		fmt.Printf("Page %d: Downloading %d images...\n", page, len(taggedImages))

		// Download images
		for _, img := range taggedImages {
			if img.CreatedAt.Before(oldest) || oldest.IsZero() {
				oldest = img.CreatedAt
			}

			createdAtUtc := img.CreatedAt.UTC()
			dateTime := createdAtUtc.Format("20060102_150405")
			milliseconds := fmt.Sprintf("%03d", createdAtUtc.Nanosecond()/1e6)
			dateTimeStr := dateTime + milliseconds

			// skip if image already downloaded
			if _, exists := downloadedIDs[img.ImageID]; exists {
				fmt.Printf("Image %s already downloaded, skipping...\n", img.ImageID)
				continue
			}

			fullSizedImageUrl := fmt.Sprintf("%s/%dx%d/%s", img.Prefix, img.Width, img.Height, img.Key)
			resp, err := http.Get(fullSizedImageUrl)
			if err != nil {
				fmt.Println("Error downloading image:", err)
				continue
			}
			if resp.StatusCode != http.StatusOK {
				fmt.Printf("Failed to download image %s: %s\n", img.ImageID, resp.Status)
				resp.Body.Close()
				continue
			}
			if _, err := os.Stat("output"); os.IsNotExist(err) {
				err = os.Mkdir("output", 0755)
				if err != nil {
					fmt.Println("Error creating output directory:", err)
					resp.Body.Close()
					continue
				}
			}
			fileName := fmt.Sprintf("output/%s_%s.jpg", dateTimeStr, img.ImageID)
			outFile, err := os.Create(fileName)
			if err != nil {
				fmt.Println("Error creating output file:", err)
				resp.Body.Close()
				continue
			}
			_, err = io.Copy(outFile, resp.Body)
			resp.Body.Close()
			outFile.Close()
			if err != nil {
				fmt.Println("Error saving image to file:", err)
			}

			// Update EXIF data with createdAt and location using go-exif
			err = updateExifData(fileName, img.CreatedAt, opts.Latitude, opts.Longitude)
			if err != nil {
				fmt.Println("Error writing EXIF data to file:", err)
				continue
			}

			fmt.Printf("Image %s saved to %s\n", img.ImageID, fileName)
			downloadedIDs[img.ImageID] = time.Now()
		}

		page++
	}

	fmt.Println("All done here.")
}

// updateExifData updates the EXIF DateTimeOriginal and GPS tags in a JPEG file.
func updateExifData(imagePath string, createdAt time.Time, latitude, longitude float64) error {
	// Check if exiftool is available
	if _, err := exec.LookPath("exiftool"); err != nil {
		return fmt.Errorf("exiftool not found: %w", err)
	}

	// Format datetime for EXIF (YYYY:MM:DD HH:MM:SS)
	dateTimeStr := createdAt.Format("2006:01:02 15:04:05")
	offsetStr := createdAt.Format("-07:00")

	// Convert coordinates to EXIF format
	latRef := "N"
	lat := latitude
	if lat < 0 {
		latRef = "S"
		lat = -lat
	}

	lonRef := "E"
	lon := longitude
	if lon < 0 {
		lonRef = "W"
		lon = -lon
	}

	// Build exiftool command
	cmd := exec.Command("exiftool",
		"-overwrite_original",
		"-DateTimeOriginal="+dateTimeStr,
		"-OffsetTimeOriginal="+offsetStr,
		"-SubSecTimeOriginal=000",
		"-CreateDate="+dateTimeStr,
		"-ModifyDate="+dateTimeStr,
		fmt.Sprintf("-GPSLatitude=%f", lat),
		"-GPSLatitudeRef="+latRef,
		fmt.Sprintf("-GPSLongitude=%f", lon),
		"-GPSLongitudeRef="+lonRef,
		imagePath,
	)

	// Execute command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("exiftool failed: %w, output: %s", err, string(output))
	}

	return nil
}

func loadState() (map[string]time.Time, error) {
	ids := make(map[string]time.Time)
	data, err := os.ReadFile("state.json")
	if os.IsNotExist(err) {
		return ids, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return ids, nil
	}
	if err := json.Unmarshal(data, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func saveState(downloadedIDs map[string]time.Time) error {
	data, err := json.MarshalIndent(downloadedIDs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile("state.json", data, 0644)
}
