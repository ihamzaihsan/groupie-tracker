package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type application struct {
	api       apiClient
	templates *template.Template
}

type ErrorPageData struct {
	Code     int
	ErrorMsg string
}

type detailPageData struct {
	Artist             Artist
	ProcessedLocations []string
	ProcessedDates     []string
	Relation           Relation
}

func newApplication(client *http.Client, apiURL string) (*application, error) {
	templates, err := template.New("pages").Funcs(template.FuncMap{
		"location": formatLocation,
		"date":     formatDate,
	}).ParseFS(resources, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	return &application{api: apiClient{client: client, baseURL: strings.TrimRight(apiURL, "/")}, templates: templates}, nil
}

func formatLocation(value string) string {
	parts := strings.Split(strings.ReplaceAll(value, "_", " "), "-")
	for i, part := range parts {
		words := strings.Fields(part)
		for j, word := range words {
			if word == "usa" || word == "uk" {
				words[j] = strings.ToUpper(word)
				continue
			}
			letters := []rune(word)
			words[j] = strings.ToUpper(string(letters[0])) + string(letters[1:])
		}
		parts[i] = strings.Join(words, " ")
	}
	return strings.Join(parts, " · ")
}

func formatDate(value string) string {
	parsed, err := time.Parse("02-01-2006", strings.TrimPrefix(value, "*"))
	if err != nil {
		return value
	}
	return parsed.Format("02 Jan 2006")
}

// Render before committing headers so template failures can return a clean 500.
func (app *application) render(w http.ResponseWriter, status int, name string, data any) {
	var page bytes.Buffer
	if err := app.templates.ExecuteTemplate(&page, name, data); err != nil {
		log.Printf("render %s: %v", name, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if _, err := page.WriteTo(w); err != nil {
		log.Printf("write response: %v", err)
	}
}

func (app *application) errorPage(w http.ResponseWriter, status int, message string) {
	app.render(w, status, "error.html", ErrorPageData{Code: status, ErrorMsg: message})
}

func (app *application) allowGet(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet {
		return true
	}
	w.Header().Set("Allow", http.MethodGet)
	app.errorPage(w, http.StatusMethodNotAllowed, "Method not allowed")
	return false
}

func (app *application) apiFailure(w http.ResponseWriter, err error) {
	log.Printf("API request failed: %v", err)
	app.errorPage(w, http.StatusBadGateway, "Artist data is temporarily unavailable. Please try again later.")
}

func (app *application) artists(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		app.errorPage(w, http.StatusNotFound, "Page not found")
		return
	}
	if !app.allowGet(w, r) {
		return
	}
	app.searchArtists(w, r)
}

func (app *application) artistDetails(w http.ResponseWriter, r *http.Request) {
	if !app.allowGet(w, r) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	rawID := strings.TrimPrefix(r.URL.Path, "/artist/")
	id, err := strconv.Atoi(rawID)
	if err != nil || id < 1 || rawID != strconv.Itoa(id) {
		app.errorPage(w, http.StatusBadRequest, "Artist ID must be a positive integer")
		return
	}
	var data detailPageData
	if err := app.api.fetch(r.Context(), "/artists/"+strconv.Itoa(id), &data.Artist); err != nil {
		var upstream *upstreamError
		if errors.As(err, &upstream) && upstream.status == http.StatusNotFound {
			app.errorPage(w, http.StatusNotFound, "Artist not found")
		} else {
			app.apiFailure(w, err)
		}
		return
	}
	// Some APIs return an empty object instead of a 404 for an unknown ID.
	if data.Artist.ID == 0 {
		app.errorPage(w, http.StatusNotFound, "Artist not found")
		return
	}
	if data.Artist.ID != id {
		app.apiFailure(w, fmt.Errorf("artist ID mismatch: requested %d, got %d", id, data.Artist.ID))
		return
	}
	var locations struct {
		Index []LocationDetails `json:"index"`
	}
	var dates struct {
		Index []Date `json:"index"`
	}
	var relations struct {
		Index []Relation `json:"index"`
	}
	for _, request := range []struct {
		path        string
		destination any
	}{
		{"/locations", &locations}, {"/dates", &dates}, {"/relation", &relations},
	} {
		if err := app.api.fetch(r.Context(), request.path, request.destination); err != nil {
			app.apiFailure(w, err)
			return
		}
	}
	for _, location := range locations.Index {
		if location.ID == id {
			data.ProcessedLocations = location.Locations
			break
		}
	}
	for _, date := range dates.Index {
		if date.ID == id {
			data.ProcessedDates = date.Dates
			break
		}
	}
	for _, relation := range relations.Index {
		if relation.ID == id {
			data.Relation = relation
			break
		}
	}
	app.render(w, http.StatusOK, "details.html", data)
}
