package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"time"
)

//go:embed templates/*.html assets/*.css assets/*.svg assets/*.js data/*.json
var resources embed.FS

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	geocodeURL := flag.String("geocode-url", "https://photon.komoot.io/api/", "Photon-compatible geocoding endpoint")
	geocodeCache := flag.String("geocode-cache", ".cache/geocodes.json", "Geocoding cache file (empty disables disk caching)")
	refreshMapData := flag.String("refresh-map-data", "", "Geocode all current concert locations into a JSON snapshot and exit")
	checkMapData := flag.Bool("check-map-data", false, "Check bundled coordinate coverage against the current concert API and exit")
	flag.Parse()
	app, err := newApplication(&http.Client{Timeout: 10 * time.Second}, baseURL)
	if err != nil {
		log.Fatal(err)
	}
	app.geocoder, err = newGeocoder(*geocodeURL, *geocodeCache)
	if err != nil {
		log.Fatal(err)
	}
	if *refreshMapData != "" || *checkMapData {
		if *refreshMapData != "" && *checkMapData {
			log.Fatal("choose either refresh-map-data or check-map-data")
		}
		if err := app.auditMapData(*refreshMapData); err != nil {
			log.Fatal(err)
		}
		return
	}
	assets, err := fs.Sub(resources, "assets")
	if err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assets))))
	mux.HandleFunc("/artist/", app.artistDetails)
	mux.HandleFunc("/", app.artists)
	server := &http.Server{
		Addr: *addr, Handler: mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      45 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("Server listening on %s (Ctrl+C to stop)", *addr)
	log.Fatal(server.ListenAndServe())
}
