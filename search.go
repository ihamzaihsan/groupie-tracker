package main

import (
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

type searchSuggestion struct {
	Value    string
	Category string
	Input    string
}

type collectionPageData struct {
	Artists     []Artist
	Total       int
	Query       string
	Sort        string
	View        string
	Suggestions []searchSuggestion
	Filters     artistFilters
}

type searchField struct {
	value    string
	category string
}

// Normalize API location separators and whitespace as well as letter case.
func normalizeSearch(value string) string {
	value = strings.NewReplacer("_", " ", "-", " ", "·", " ").Replace(value)
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func artistSearchFields(artist Artist, locations []string) []searchField {
	fields := []searchField{
		{artist.Name, "artist/band"},
		{artist.FirstAlbum, "first album date"},
		{strconv.Itoa(artist.CreationDate), "creation date"},
	}
	for _, member := range artist.Members {
		fields = append(fields, searchField{member, "member"})
	}
	for _, location := range locations {
		fields = append(fields, searchField{formatLocation(location), "location"})
	}
	return fields
}

func buildSuggestions(fieldsByArtist map[int][]searchField) []searchSuggestion {
	seen := make(map[string]bool)
	var suggestions []searchSuggestion
	for _, fields := range fieldsByArtist {
		for _, field := range fields {
			if strings.TrimSpace(field.value) == "" {
				continue
			}
			input := field.value + " — " + field.category
			key := strings.ToLower(input)
			if seen[key] {
				continue
			}
			seen[key] = true
			suggestions = append(suggestions, searchSuggestion{Value: field.value, Category: field.category, Input: input})
		}
	}
	sort.Slice(suggestions, func(i, j int) bool {
		return strings.ToLower(suggestions[i].Input) < strings.ToLower(suggestions[j].Input)
	})
	return suggestions
}

func matchesArtist(fields []searchField, query, category string) bool {
	for _, field := range fields {
		if category != "" && category != field.category {
			continue
		}
		if strings.Contains(normalizeSearch(field.value), query) {
			return true
		}
		if field.category == "first album date" && strings.Contains(normalizeSearch(formatDate(field.value)), query) {
			return true
		}
	}
	return false
}

func (app *application) searchArtists(w http.ResponseWriter, r *http.Request) {
	if len(r.URL.RawQuery) > 16384 {
		app.errorPage(w, http.StatusBadRequest, "Collection parameters are too long")
		return
	}
	params, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		app.errorPage(w, http.StatusBadRequest, "Invalid search parameters")
		return
	}
	filters, err := parseArtistFilters(params)
	if err != nil {
		app.errorPage(w, http.StatusBadRequest, err.Error())
		return
	}
	query := strings.TrimSpace(params.Get("q"))
	if !utf8.ValidString(query) || utf8.RuneCountInString(query) > 200 {
		app.errorPage(w, http.StatusBadRequest, "Search must contain at most 200 characters")
		return
	}
	sortOrder, view := params.Get("sort"), params.Get("view")
	if sortOrder == "" {
		sortOrder = "original"
	}
	if view == "" {
		view = "grid"
	}
	if (sortOrder != "original" && sortOrder != "name" && sortOrder != "oldest" && sortOrder != "newest") || (view != "grid" && view != "list") {
		app.errorPage(w, http.StatusBadRequest, "Invalid collection display option")
		return
	}
	var artists []Artist
	if err := app.api.fetch(r.Context(), "/artists", &artists); err != nil {
		app.apiFailure(w, err)
		return
	}
	var locations struct {
		Index []LocationDetails `json:"index"`
	}
	if err := app.api.fetch(r.Context(), "/locations", &locations); err != nil {
		app.apiFailure(w, err)
		return
	}
	locationsByID := make(map[int][]string, len(locations.Index))
	for _, location := range locations.Index {
		locationsByID[location.ID] = location.Locations
	}
	if err := filters.buildOptions(artists, locationsByID, app.geocoder.catalog); err != nil {
		app.errorPage(w, http.StatusBadRequest, err.Error())
		return
	}
	fieldsByArtist := make(map[int][]searchField, len(artists))
	for _, artist := range artists {
		fieldsByArtist[artist.ID] = artistSearchFields(artist, locationsByID[artist.ID])
	}
	data := collectionPageData{
		Total: len(artists), Query: query, Sort: sortOrder, View: view,
		Suggestions: buildSuggestions(fieldsByArtist),
		Filters:     filters,
	}
	searchText, category := query, ""
	// A selected native suggestion restricts the search to its labeled category.
	for _, suggestion := range data.Suggestions {
		if strings.EqualFold(query, suggestion.Input) {
			searchText, category = suggestion.Value, suggestion.Category
			break
		}
	}
	normalized := normalizeSearch(searchText)
	for _, artist := range artists {
		if (normalized == "" || matchesArtist(fieldsByArtist[artist.ID], normalized, category)) && filters.matches(artist, locationsByID[artist.ID], app.geocoder.catalog) {
			data.Artists = append(data.Artists, artist)
		}
	}
	sort.SliceStable(data.Artists, func(i, j int) bool {
		a, b := data.Artists[i], data.Artists[j]
		switch sortOrder {
		case "name":
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		case "oldest":
			return a.CreationDate < b.CreationDate
		case "newest":
			return a.CreationDate > b.CreationDate
		default:
			return false
		}
	})
	app.render(w, http.StatusOK, "index.html", data)
}
