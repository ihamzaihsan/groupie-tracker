package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"net/url"
	"sort"
	"time"
)

type concertMapLocation struct {
	Key          string
	Label        string
	Number       int
	Found        bool
	X            string
	Y            string
	Coordinates  string
	MatchedPlace string
	DetailURL    string
	Problem      string
	Note         string
}

type concertMapData struct {
	Loaded       bool
	Locations    []concertMapLocation
	Mapped       int
	Unavailable  int
	Focus        string
	FocusedName  string
	ViewBox      string
	MarkerRadius string
	LabelSize    string
}

func (app *application) buildConcertMap(ctx context.Context, locations []string, relations map[string][]string, loaded bool, focus string) (concertMapData, error) {
	keys := make(map[string]bool)
	for _, location := range locations {
		keys[location] = true
	}
	for location := range relations {
		keys[location] = true
	}
	if focus != "" && !keys[focus] {
		return concertMapData{}, errors.New("focus is not one of the artist's concert locations")
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	data := concertMapData{Loaded: loaded, Focus: focus, ViewBox: "0 0 1000 500", MarkerRadius: "7", LabelSize: "6.5"}
	lookupCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	for i, key := range ordered {
		location := concertMapLocation{Key: key, Label: formatLocation(key), Number: i + 1}
		if loaded {
			var point geographicPoint
			var err error
			switch {
			case app.geocoder == nil:
				err = errors.New("geocoder is unavailable")
			case lookupCtx.Err() != nil:
				err = lookupCtx.Err()
			default:
				point, err = app.geocoder.locate(lookupCtx, key)
			}
			if err != nil {
				location.Problem = "Coordinates temporarily unavailable."
				if errors.Is(err, errLocationNotFound) {
					location.Problem = "No geographic match found."
				}
				if errors.Is(err, context.DeadlineExceeded) {
					location.Problem = "Lookup time limit reached. Refresh to continue."
				}
				log.Printf("map location %s: %v", key, err)
				data.Unavailable++
			} else {
				location.Found = true
				data.Mapped++
				location.MatchedPlace = point.MatchedPlace
				location.Note = point.Note
				location.Coordinates = fmt.Sprintf("%.4f°, %.4f°", point.Latitude, point.Longitude)
				// Same equirectangular projection as the embedded Natural Earth basemap.
				x, y := (point.Longitude+180)/360*1000, (90-point.Latitude)/180*500
				location.X, location.Y = fmt.Sprintf("%.3f", x), fmt.Sprintf("%.3f", y)
				params := url.Values{"mlat": {fmt.Sprintf("%.6f", point.Latitude)}, "mlon": {fmt.Sprintf("%.6f", point.Longitude)}}
				location.DetailURL = "https://www.openstreetmap.org/?" + params.Encode() + fmt.Sprintf("#map=12/%.6f/%.6f", point.Latitude, point.Longitude)
				if key == focus {
					data.FocusedName = location.Label
					data.ViewBox = fmt.Sprintf("%.3f %.3f 140 70", math.Max(0, math.Min(860, x-70)), math.Max(0, math.Min(430, y-35)))
					data.MarkerRadius, data.LabelSize = "4", "3"
				}
			}
		}
		data.Locations = append(data.Locations, location)
	}
	return data, nil
}
