package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"text/template"
)

const (
	baseURL       = "https://groupietrackers.herokuapp.com/api"
	artistsPath   = "/artists"
	locationsPath = "/locations"
	datesPath     = "/dates"
	relationPath  = "/relation"
)

type Artist struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	Image        string   `json:"image"`
	Members      []string `json:"members"`
	CreationDate int      `json:"creationDate"`
	FirstAlbum   string   `json:"firstAlbum"`
}

type LocationDetails struct {
	ID        int      `json:"id"`
	Locations []string `json:"locations"`
}

type LocationsResponse struct {
	Locations []LocationDetails `json:"index"`
}

type Date struct {
	ID    int      `json:"id"`
	Dates []string `json:"dates"`
}

type DatesResponse struct {
	Dates []Date `json:"index"`
}

type Relation struct {
	ID             int                 `json:"id"`
	DatesLocations map[string][]string `json:"datesLocations"`
}

type RelationsResponse struct {
	Relations []Relation `json:"index"`
}

// Function to fetch data from a URL
func fetchData(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// Function to get the full URL
func getFullURL(path string) string {
	return baseURL + path
}

// Existing functions to get locations, dates, and relations
func getLocations() ([]LocationDetails, error) {
	data, err := fetchData(getFullURL(locationsPath))
	if err != nil {
		log.Printf("Error fetching locations: %v", err)
		return nil, err
	}

	var locationsResponse LocationsResponse
	if err := json.Unmarshal(data, &locationsResponse); err != nil {
		log.Printf("Error unmarshalling locations: %v", err)
		return nil, err
	}

	return locationsResponse.Locations, nil
}

func getDates() ([]Date, error) {
	data, err := fetchData(getFullURL(datesPath))
	if err != nil {
		log.Printf("Error fetching dates: %v", err)
		return nil, err
	}

	var datesResponse DatesResponse
	if err := json.Unmarshal(data, &datesResponse); err != nil {
		log.Printf("Error unmarshalling dates: %v", err)
		return nil, err
	}

	return datesResponse.Dates, nil
}

func getRelations() ([]Relation, error) {
	data, err := fetchData(getFullURL(relationPath))
	if err != nil {
		log.Printf("Error fetching relations: %v", err)
		return nil, err
	}

	var relationsResponse RelationsResponse
	if err := json.Unmarshal(data, &relationsResponse); err != nil {
		log.Printf("Error unmarshalling relations: %v", err)
		return nil, err
	}

	return relationsResponse.Relations, nil
}

// New function to get artist details
func getArtistDetails(w http.ResponseWriter, r *http.Request) {
	artistID := r.URL.Query().Get("id")
	if artistID == "" {
		http.Error(w, "Missing artist ID", http.StatusBadRequest)
		return
	}

	// Fetch artist details
	artistData, err := fetchData(getFullURL(artistsPath + "/" + artistID))
	if err != nil {
		log.Printf("Error fetching artist: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var artist Artist
	if err := json.Unmarshal(artistData, &artist); err != nil {
		log.Printf("Error unmarshalling artist: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch locations
	locations, err := getLocations()
	if err != nil {
		log.Printf("Error fetching locations: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch dates
	dates, err := getDates()
	if err != nil {
		log.Printf("Error fetching dates: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch relations
	relations, err := getRelations()
	if err != nil {
		log.Printf("Error fetching relations: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Filter data for the specific artist
	var location LocationDetails
	for _, loc := range locations {
		if loc.ID == artist.ID {
			location = loc
			break
		}
	}

	var date Date
	for _, d := range dates {
		if d.ID == artist.ID {
			date = d
			break
		}
	}

	var relation Relation
	for _, rel := range relations {
		if rel.ID == artist.ID {
			relation = rel
			break
		}
	}

	// Render the template
	tmpl, err := template.ParseFiles("frontend/artists.html")
	if err != nil {
		log.Printf("Error parsing template file: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Artist   Artist
		Location LocationDetails
		Date     Date
		Relation Relation
	}{
		Artist:   artist,
		Location: location,
		Date:     date,
		Relation: relation,
	}

	fmt.Printf("Adata.Location: %v\n", data.Location)
    fmt.Printf("Adata.date: %v\n", data.Date)
    fmt.Printf("Adata.relation: %v\n", data.Relation)

	w.Header().Set("Content-Type", "text/html")
	tmpl.Execute(w, data)
}

// Function to get artists and render the index page
func getArtists(w http.ResponseWriter, r *http.Request) {
	data, err := fetchData(getFullURL(artistsPath))
	if err != nil {
		log.Printf("Error fetching artists: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var artists []Artist
	if err := json.Unmarshal(data, &artists); err != nil {
		log.Printf("Error unmarshalling artists: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl, err := template.ParseFiles("frontend/index.html")
	if err != nil {
		log.Printf("Error parsing template file: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	tmpl.Execute(w, artists)
}

// Main function
func main() {
	http.HandleFunc("/", getArtists)
	http.HandleFunc("/artist", getArtistDetails)

	log.Println("Server started at :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
