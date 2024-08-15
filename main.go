package main

import (
    "encoding/json"
    "io"
    "log"
    "net/http"
    "strconv"
    "strings"
    "text/template"
    "fmt"
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

type ErrorPageData struct {
    Code     string
    ErrorMsg string
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

// Function to render the error page
func renderErrorPage(w http.ResponseWriter, code int, errorMsg string) {
    w.WriteHeader(code)
    tmpl, err := template.ParseFiles("templates/error.html")
    if err != nil {
        log.Printf("Error parsing error template: %v", err)
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
        return
    }
    data := ErrorPageData{
        Code:     strconv.Itoa(code),
        ErrorMsg: errorMsg,
    }
    tmpl.Execute(w, data)
}

// Custom NotFoundHandler to render the error page
func notFoundHandler(w http.ResponseWriter, r *http.Request) {
    renderErrorPage(w, http.StatusNotFound, "Page not found")
}

// Existing functions to get locations, dates, and relations
func getLocations() ([]LocationDetails, error) {
    data, err := fetchData(getFullURL(locationsPath))
    if err != nil {
        return nil, err
    }

    var locationsResponse LocationsResponse
    err = json.Unmarshal(data, &locationsResponse)
    if err != nil {
        return nil, err
    }

    return locationsResponse.Locations, nil
}

func getDates() ([]Date, error) {
    data, err := fetchData(getFullURL(datesPath))
    if err != nil {
        return nil, err
    }

    var datesResponse DatesResponse
    err = json.Unmarshal(data, &datesResponse)
    if err != nil {
        return nil, err
    }

    return datesResponse.Dates, nil
}

func getRelations() ([]Relation, error) {
    data, err := fetchData(getFullURL(relationPath))
    if err != nil {
        return nil, err
    }

    var relationsResponse RelationsResponse
    err = json.Unmarshal(data, &relationsResponse)
    if err != nil {
        return nil, err
    }

    return relationsResponse.Relations, nil
}

// New function to get artist details
func getArtistDetails(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        renderErrorPage(w, http.StatusMethodNotAllowed, "Method not allowed")
        return
    }

    // Extract artist ID from the URL path
    artistID := strings.TrimPrefix(r.URL.Path, "/artist/")
    if artistID == "" {
        renderErrorPage(w, http.StatusBadRequest, "Missing artist ID")
        return
    }

    // Convert artistID to an integer
    id, err := strconv.Atoi(artistID)
    if err != nil || id < 1 || id > 52 {
        renderErrorPage(w, http.StatusBadRequest, "Artist not Found")
        return
    }

    // Fetch artist details
    artistData, err := fetchData(getFullURL(artistsPath + "/" + artistID))
    if err != nil {
        log.Printf("Error fetching artist: %v", err)
        renderErrorPage(w, http.StatusInternalServerError, "Error fetching artist details")
        return
    }

    var artist Artist
    err = json.Unmarshal(artistData, &artist)
    if err != nil {
        log.Printf("Error unmarshalling artist data: %v", err)
        renderErrorPage(w, http.StatusInternalServerError, "Error processing artist details")
        return
    }

    // Fetch locations
    locations, err := getLocations()
    if err != nil {
        log.Printf("Error fetching locations: %v", err)
        renderErrorPage(w, http.StatusInternalServerError, "Error fetching locations")
        return
    }

    // Fetch dates
    dates, err := getDates()
    if err != nil {
        log.Printf("Error fetching dates: %v", err)
        renderErrorPage(w, http.StatusInternalServerError, "Error fetching dates")
        return
    }

    // Fetch relations
    relations, err := getRelations()
    if err != nil {
        log.Printf("Error fetching relations: %v", err)
        renderErrorPage(w, http.StatusInternalServerError, "Error fetching relations")
        return
    }

    // Filter data for the specific artist
    var location LocationDetails
    for _, loc := range locations {
        if loc.ID == id {
            location = loc
            break
        }
    }

    var date Date
    for _, d := range dates {
        if d.ID == id {
            date = d
            break
        }
    }

    var relation Relation
    for _, rel := range relations {
        if rel.ID == id {
            relation = rel
            break
        }
    }

    // Process location data
    var processedLocations []string
    for _, loc := range location.Locations {
        processedLocations = append(processedLocations, loc)
    }

    // Process date data
    var processedDates []string
    for _, d := range date.Dates {
        processedDates = append(processedDates, d)
    }

    // Render the template
    tmpl, err := template.ParseFiles("templates/details.html")
    if err != nil {
        log.Printf("Error parsing details template: %v", err)
        renderErrorPage(w, http.StatusInternalServerError, "Error rendering artist details")
        return
    }

    data := struct {
        Artist            Artist
        ProcessedLocations []string
        ProcessedDates     []string
        Relation           Relation
    }{
        Artist:            artist,
        ProcessedLocations: processedLocations,
        ProcessedDates:     processedDates,
        Relation:           relation,
    }

    tmpl.Execute(w, data)
}

// Function to get artists and render the index page
func getArtists(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        renderErrorPage(w, http.StatusMethodNotAllowed, "Method not allowed")
        return
    }

    // Fetch artists
    artistData, err := fetchData(getFullURL(artistsPath))
    if err != nil {
        log.Printf("Error fetching artists: %v", err)
        renderErrorPage(w, http.StatusInternalServerError, "Error fetching artists")
        return
    }

    var artists []Artist
    err = json.Unmarshal(artistData, &artists)
    if err != nil {
        log.Printf("Error unmarshalling artists data: %v", err)
        renderErrorPage(w, http.StatusInternalServerError, "Error processing artists data")
        return
    }

    // Render the template
    tmpl, err := template.ParseFiles("templates/index.html")
    if err != nil {
        log.Printf("Error parsing index template: %v", err)
        renderErrorPage(w, http.StatusInternalServerError, "Error rendering artists list")
        return
    }

    tmpl.Execute(w, artists)
}

// Main function
func main() {
    // http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets/"))))
    mux := http.NewServeMux()
    mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets"))))
    mux.HandleFunc("/artist/", getArtistDetails)
    mux.HandleFunc("/", getArtists)

    // Set custom NotFoundHandler
    mux.HandleFunc("/404", notFoundHandler)

    // Custom handler to catch all undefined routes
    catchAllHandler := func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path != "/" && !strings.HasPrefix(r.URL.Path, "/artist/") && !strings.HasPrefix(r.URL.Path, "/assets/") {
            notFoundHandler(w, r)
            return
        }
        mux.ServeHTTP(w, r)
    }
    fmt.Println("Server starting: http://localhost:8080")
    fmt.Println("CTRL + C to stop server")
    log.Fatal(http.ListenAndServe(":8080", http.HandlerFunc(catchAllHandler)))
}