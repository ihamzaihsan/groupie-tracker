package main

import (
    "encoding/json"
    "github.com/gorilla/mux"
    "io"
    "log"
    "net/http"
    "strconv"
    "strings"
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
        http.Error(w, "Error 500: Internal Server Error", http.StatusInternalServerError)
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
    if r.Method != http.MethodGet {
        http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
        return
    }

    vars := mux.Vars(r)
    artistID := vars["id"]
    // if artistID == "" {
    //     renderErrorPage(w, http.StatusBadRequest, "Missing artist ID")
    //     return
    // }

    // Convert artistID to an integer
    id, err := strconv.Atoi(artistID)
    if err != nil || id < 1 || id > 52 {
        renderErrorPage(w, http.StatusNotFound, "Artist not found")
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
    if err := json.Unmarshal(artistData, &artist); err != nil {
        log.Printf("Error unmarshalling artist: %v", err)
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

    // Process location data
    var processedLocations []string
    for i, loc := range location.Locations {
        processedLoc := strings.ReplaceAll(loc, "_", " ")
        if i == len(location.Locations)-1 {
            processedLoc = strings.TrimSuffix(processedLoc, ",") + "."
        }
        processedLocations = append(processedLocations, processedLoc)
    }

    // Process date data
    var processedDates []string
    for i, d := range date.Dates {
        processedDate := strings.Replace(d, "*", ",", -1)
        if i == 0 {
            processedDate = strings.TrimPrefix(processedDate, ",")
        }
        if i == len(date.Dates)-1 {
            processedDate = strings.TrimSuffix(processedDate, ",") + "."
        }
        processedDates = append(processedDates, processedDate)
    }

    // Render the template
    tmpl, err := template.ParseFiles("templates/details.html")
    if err != nil {
        log.Printf("Error parsing template file: %v", err)
        renderErrorPage(w, http.StatusInternalServerError, "Error rendering page")
        return
    }

    data := struct {
        Artist             Artist
        ProcessedLocations []string
        ProcessedDates     []string
        Relation           Relation
    }{
        Artist:             artist,
        ProcessedLocations: processedLocations,
        ProcessedDates:     processedDates,
        Relation:           relation,
    }

    w.Header().Set("Content-Type", "text/html")
    tmpl.Execute(w, data)
}

// Function to get artists and render the index page
func getArtists(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
        return
    }

    data, err := fetchData(getFullURL(artistsPath))
    if err != nil {
        log.Printf("Error fetching artists: %v", err)
        renderErrorPage(w, http.StatusInternalServerError, "Error fetching artists")
        return
    }

    var artists []Artist
    if err := json.Unmarshal(data, &artists); err != nil {
        log.Printf("Error unmarshalling artists: %v", err)
        renderErrorPage(w, http.StatusInternalServerError, "Error processing artists")
        return
    }

    tmpl, err := template.ParseFiles("templates/index.html")
    if err != nil {
        log.Printf("Error parsing template file: %v", err)
        renderErrorPage(w, http.StatusInternalServerError, "Error rendering page")
        return
    }

    w.Header().Set("Content-Type", "text/html")
    if err := tmpl.Execute(w, artists); err != nil {
        log.Printf("Error executing template: %v", err)
        renderErrorPage(w, http.StatusInternalServerError, "Error rendering page")
    }
}

// Main function
func main() {
    r := mux.NewRouter()
    r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets/"))))
    r.HandleFunc("/", getArtists)
    r.HandleFunc("/artist/{id}", getArtistDetails)
	r.NotFoundHandler = http.HandlerFunc(notFoundHandler)
    log.Println("Server started at http://localhost:8080")
    log.Fatal(http.ListenAndServe(":8080", r))
}