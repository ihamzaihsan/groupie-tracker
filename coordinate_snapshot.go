package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

type coordinateSnapshot struct {
	GeneratedAt time.Time                  `json:"generatedAt"`
	Provider    string                     `json:"provider"`
	License     string                     `json:"license"`
	Locations   map[string]geographicPoint `json:"locations"`
}

func readCoordinateSnapshot() (coordinateSnapshot, error) {
	var snapshot coordinateSnapshot
	body, err := resources.ReadFile("data/concert-locations.json")
	if err != nil {
		return snapshot, err
	}
	if err := json.Unmarshal(body, &snapshot); err != nil {
		return snapshot, fmt.Errorf("read coordinate snapshot: %w", err)
	}
	if snapshot.Locations == nil {
		return snapshot, errors.New("coordinate snapshot must contain a locations object")
	}
	for key, point := range snapshot.Locations {
		if key == "" || !point.valid() || point.MatchedPlace == "" {
			return snapshot, fmt.Errorf("invalid snapshot coordinates for %q", key)
		}
	}
	return snapshot, nil
}

// Handles spelling and transliteration differences in this API's place names.
func normalizeGeocodeName(value string) string {
	value = strings.ToLower(strings.ReplaceAll(value, "_", " "))
	if value == "napoca" {
		value = "cluj-napoca"
	}
	if value == "burriana" {
		value = "borriana"
	}
	if strings.HasPrefix(value, "st ") {
		value = "saint " + value[3:]
	}
	if strings.HasPrefix(value, "st. ") {
		value = "saint " + value[4:]
	}
	value = strings.NewReplacer("á", "a", "à", "a", "â", "a", "ä", "a", "ã", "a", "å", "a", "é", "e", "è", "e", "ê", "e", "ë", "e", "í", "i", "ì", "i", "î", "i", "ï", "i", "ó", "o", "ò", "o", "ô", "o", "ö", "o", "õ", "o", "ø", "o", "ú", "u", "ù", "u", "û", "u", "ü", "u", "ç", "c", "ñ", "n", "ß", "ss").Replace(value)
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return -1
	}, value)
}

// A maintenance command, separate from browser requests. It never publishes a
// partial snapshot over the last complete one and does not geocode during checks.
func (app *application) auditMapData(outputPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	var locations struct {
		Index []LocationDetails `json:"index"`
	}
	var relations struct {
		Index []Relation `json:"index"`
	}
	if err := app.api.fetch(ctx, "/locations", &locations); err != nil {
		return err
	}
	if err := app.api.fetch(ctx, "/relation", &relations); err != nil {
		return err
	}
	if len(locations.Index) == 0 || len(relations.Index) == 0 {
		return errors.New("cannot audit an empty concert API response")
	}
	keys := make(map[string]bool)
	for _, artist := range locations.Index {
		for _, key := range artist.Locations {
			keys[key] = true
		}
	}
	for _, artist := range relations.Index {
		for key := range artist.DatesLocations {
			keys[key] = true
		}
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	points := make(map[string]geographicPoint, len(keys))
	var missing []string
	reviewed := app.geocoder.catalog
	if outputPath != "" {
		app.geocoder.catalog = nil
	}
	for i, key := range ordered {
		point, found := app.geocoder.catalog[key]
		if outputPath != "" {
			var err error
			point, err = app.geocoder.locate(ctx, key)
			found = err == nil
			if err != nil {
				log.Printf("unresolved %s: %v", key, err)
			}
		}
		if !found || !point.valid() {
			missing = append(missing, key)
		} else {
			if previous, ok := reviewed[key]; ok {
				point.Note = previous.Note
			}
			points[key] = point
		}
		if outputPath != "" {
			log.Printf("map coverage %d/%d: %s", i+1, len(ordered), key)
		}
	}
	log.Printf("Coordinate coverage: %d/%d unique locations across %d artist location records", len(points), len(keys), len(locations.Index))
	if len(missing) > 0 {
		return fmt.Errorf("missing coordinates for: %s", strings.Join(missing, ", "))
	}
	if outputPath == "" {
		return nil
	}
	snapshot := coordinateSnapshot{GeneratedAt: time.Now().UTC(), Provider: "Photon / OpenStreetMap", License: "ODbL-1.0", Locations: points}
	body, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(outputPath), "concert-locations-*.tmp")
	if err != nil {
		return fmt.Errorf("create coordinate snapshot: %w", err)
	}
	defer os.Remove(file.Name())
	_, writeErr := file.Write(append(body, '\n'))
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(file.Name(), outputPath); err != nil {
		return fmt.Errorf("publish coordinate snapshot: %w", err)
	}
	log.Printf("Saved complete snapshot to %s; rebuild the application to embed it", outputPath)
	return nil
}
