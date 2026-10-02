package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var errLocationNotFound = errors.New("location not found")

type geographicPoint struct {
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	MatchedPlace string  `json:"matchedPlace"`
	SourceURL    string  `json:"sourceURL,omitempty"`
	Note         string  `json:"note,omitempty"`
}

func (p geographicPoint) valid() bool {
	return !math.IsNaN(p.Latitude) && !math.IsNaN(p.Longitude) && !math.IsInf(p.Latitude, 0) && !math.IsInf(p.Longitude, 0) && math.Abs(p.Latitude) <= 90 && math.Abs(p.Longitude) <= 180
}

type geocodeEntry struct {
	Point   geographicPoint `json:"point"`
	Found   bool            `json:"found"`
	Expires time.Time       `json:"expires"`
}

type geocoder struct {
	endpoint    string
	cachePath   string
	client      *http.Client
	gate        chan struct{} // Serializes cache access and provider requests across users.
	cache       map[string]geocodeEntry
	catalog     map[string]geographicPoint
	lastRequest time.Time
	retryAfter  time.Time
}

func newGeocoder(endpoint, cachePath string) (*geocoder, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
		return nil, errors.New("geocode-url must be an absolute HTTP or HTTPS endpoint without credentials")
	}
	g := &geocoder{endpoint: endpoint, cachePath: cachePath, client: &http.Client{Timeout: 5 * time.Second}, gate: make(chan struct{}, 1), cache: make(map[string]geocodeEntry)}
	snapshot, err := readCoordinateSnapshot()
	if err != nil {
		return nil, err
	}
	g.catalog = snapshot.Locations
	if cachePath == "" {
		return g, nil
	}
	file, err := os.Open(cachePath)
	if errors.Is(err, os.ErrNotExist) {
		return g, nil
	}
	if err != nil {
		log.Printf("geocode cache unavailable: %v", err)
		return g, nil
	}
	defer file.Close()
	if err := json.NewDecoder(io.LimitReader(file, maxResponseBytes)).Decode(&g.cache); err != nil {
		log.Printf("ignoring invalid geocode cache: %v", err)
		g.cache = make(map[string]geocodeEntry)
	}
	if g.cache == nil {
		g.cache = make(map[string]geocodeEntry)
	}
	for key, entry := range g.cache {
		if time.Now().After(entry.Expires) || (entry.Found && !entry.Point.valid()) {
			delete(g.cache, key)
		}
	}
	return g, nil
}

// Only called while holding gate. A failed disk write does not discard memory results.
func (g *geocoder) saveCache() {
	if g.cachePath == "" {
		return
	}
	dir := filepath.Dir(g.cachePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("create geocode cache: %v", err)
		return
	}
	file, err := os.CreateTemp(dir, "geocodes-*.tmp")
	if err != nil {
		log.Printf("create geocode cache file: %v", err)
		return
	}
	defer os.Remove(file.Name())
	err = json.NewEncoder(file).Encode(g.cache)
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(file.Name(), g.cachePath)
	}
	if err != nil {
		log.Printf("save geocode cache: %v", err)
	}
}

func (g *geocoder) locate(ctx context.Context, location string) (geographicPoint, error) {
	// This immutable, API-derived snapshot covers the current concert dataset.
	if point, ok := g.catalog[location]; ok {
		return point, nil
	}
	select {
	case g.gate <- struct{}{}:
		defer func() { <-g.gate }()
	case <-ctx.Done():
		return geographicPoint{}, ctx.Err()
	}
	key := "v2|" + g.endpoint + "|" + location
	if entry, ok := g.cache[key]; ok && time.Now().Before(entry.Expires) {
		if !entry.Found {
			return geographicPoint{}, errLocationNotFound
		}
		return entry.Point, nil
	}
	if time.Now().Before(g.retryAfter) {
		return geographicPoint{}, errors.New("geocoding service is cooling down")
	}
	if wait := time.Until(g.lastRequest.Add(time.Second)); wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return geographicPoint{}, ctx.Err()
		}
	}
	g.lastRequest = time.Now()
	point, err := g.fetch(ctx, location)
	if err != nil && !errors.Is(err, errLocationNotFound) {
		if ctx.Err() == nil {
			g.retryAfter = time.Now().Add(30 * time.Second)
		}
		return geographicPoint{}, err
	}
	entry := geocodeEntry{Point: point, Found: err == nil, Expires: time.Now().Add(30 * 24 * time.Hour)}
	if err != nil {
		entry.Expires = time.Now().Add(24 * time.Hour)
	}
	g.cache[key] = entry
	g.saveCache()
	return point, err
}

func (g *geocoder) fetch(ctx context.Context, location string) (geographicPoint, error) {
	u, _ := url.Parse(g.endpoint) // Validated at startup.
	params := u.Query()
	parts := strings.SplitN(location, "-", 2)
	query := strings.ReplaceAll(parts[0], "_", " ")
	if len(parts) == 2 {
		if code := concertCountryCodes[parts[1]]; code != "" {
			params.Set("countrycode", code)
		} else {
			query += ", " + strings.ReplaceAll(parts[1], "_", " ")
		}
	}
	if len(parts) == 2 {
		switch parts[1] {
		case "new_caledonia":
			params.Set("bbox", "163,-23,169,-18")
		case "french_polynesia":
			params.Set("bbox", "-156,-29,-134,-7")
		case "netherlands_antilles":
			params.Set("bbox", "-69.3,11.8,-68.6,12.5")
		}
	}
	params.Set("q", query)
	params.Set("limit", "5")
	params.Set("lang", "en")
	for _, layer := range []string{"city", "district", "locality", "state", "county"} {
		params.Add("layer", layer)
	}
	u.RawQuery = params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return geographicPoint{}, err
	}
	req.Header.Set("User-Agent", "ConcertAtlas/1.0 (concert archive geocoding)")
	req.Header.Set("Accept", "application/json")
	resp, err := g.client.Do(req)
	if err != nil {
		return geographicPoint{}, fmt.Errorf("geocoding request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return geographicPoint{}, fmt.Errorf("geocoding returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		return geographicPoint{}, errors.New("unable to read bounded geocoding response")
	}
	var data struct {
		Type     string `json:"type"`
		Features []struct {
			Geometry struct {
				Type        string    `json:"type"`
				Coordinates []float64 `json:"coordinates"`
			} `json:"geometry"`
			Properties struct {
				Name        string `json:"name"`
				State       string `json:"state"`
				Country     string `json:"country"`
				CountryCode string `json:"countrycode"`
				OSMType     string `json:"osm_type"`
				OSMID       int64  `json:"osm_id"`
			} `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return geographicPoint{}, fmt.Errorf("decode geocoding: %w", err)
	}
	if data.Type != "FeatureCollection" {
		return geographicPoint{}, errors.New("invalid geocoding response type")
	}
	matchedIndex := -1
	// Accept only an exact normalized place-name match, including aliases.
	for i, candidate := range data.Features {
		if normalizeGeocodeName(candidate.Properties.Name) == normalizeGeocodeName(parts[0]) {
			matchedIndex = i
			break
		}
	}
	if matchedIndex < 0 {
		return geographicPoint{}, errLocationNotFound
	}
	feature := data.Features[matchedIndex]
	if feature.Geometry.Type != "Point" || len(feature.Geometry.Coordinates) != 2 {
		return geographicPoint{}, errors.New("invalid geocoding geometry")
	}
	if code := params.Get("countrycode"); code != "" && !strings.EqualFold(code, feature.Properties.CountryCode) {
		return geographicPoint{}, errors.New("geocoding returned a different country")
	}
	matchedParts := []string{feature.Properties.Name}
	if feature.Properties.State != "" && feature.Properties.State != feature.Properties.Name {
		matchedParts = append(matchedParts, feature.Properties.State)
	}
	matchedParts = append(matchedParts, feature.Properties.Country)
	point := geographicPoint{Longitude: feature.Geometry.Coordinates[0], Latitude: feature.Geometry.Coordinates[1], MatchedPlace: strings.Join(matchedParts, ", ")}
	if osmType := map[string]string{"N": "node", "W": "way", "R": "relation"}[feature.Properties.OSMType]; osmType != "" && feature.Properties.OSMID > 0 {
		point.SourceURL = fmt.Sprintf("https://www.openstreetmap.org/%s/%d", osmType, feature.Properties.OSMID)
	}
	if !point.valid() {
		return geographicPoint{}, errors.New("geocoding coordinates are outside geographic bounds")
	}
	return point, nil
}

var concertCountryCodes = map[string]string{
	"argentina": "AR", "australia": "AU", "austria": "AT", "belarus": "BY", "belgium": "BE", "brazil": "BR", "canada": "CA", "chile": "CL", "china": "CN", "colombia": "CO", "costa_rica": "CR", "czechia": "CZ", "denmark": "DK", "finland": "FI", "france": "FR", "french_polynesia": "FR", "germany": "DE", "greece": "GR", "hungary": "HU", "india": "IN", "indonesia": "ID", "ireland": "IE", "italy": "IT", "japan": "JP", "mexico": "MX", "netherlands": "NL", "netherlands_antilles": "NL", "new_caledonia": "FR", "new_zealand": "NZ", "norway": "NO", "peru": "PE", "philippines": "PH", "poland": "PL", "portugal": "PT", "qatar": "QA", "romania": "RO", "saudi_arabia": "SA", "slovakia": "SK", "south_korea": "KR", "spain": "ES", "sweden": "SE", "switzerland": "CH", "taiwan": "TW", "thailand": "TH", "uk": "GB", "united_arab_emirates": "AE", "usa": "US",
}
