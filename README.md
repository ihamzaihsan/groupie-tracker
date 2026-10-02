# Concert Atlas: Artist & Concert Explorer

A Go web application for exploring artists, bands, and concert histories. Built around the [Groupie Trackers API](https://groupietrackers.herokuapp.com/api), it combines artist profiles, categorized search, advanced filters, and concert maps in a responsive interface.

## Features

- **Artist discovery:** image cards, detailed profiles, band members, formation years, album releases, and concert histories.
- **Search:** case-insensitive matching across artists, members, locations, album dates, and creation years, with categorized typing suggestions.
- **Filtering:** date ranges, member counts, and multiple country, region, and city selections. Filters work alongside search and sorting.
- **Concert maps:** numbered SVG markers, regional focus, location details, and OpenStreetMap links.
- **Visual design:** a record-store-inspired layout, grid/list views, CSS animation, keyboard navigation, and reduced-motion support.

## Stack

**Go 1.23+**, HTML, CSS, and SVG. The backend uses Go's standard library, including `net/http`, `encoding/json`, `html/template`, and `embed`. Geocoding uses Photon with OpenStreetMap data; the basemap uses Natural Earth land outlines.

Search, filtering, and rendering run on the server. The interface uses native HTML forms and suggestions, with no application JavaScript or third-party Go dependencies.

## Run locally

From the repository root:

```sh
go run .
```

Open [localhost:8080](http://localhost:8080). Internet access is required for artist data and images. Use `go run . -addr 127.0.0.1:8081` to change the listen address, or `go run . -h` to list configuration options.

The collection supports combined search, filters, sorting, and layout selection. Artist profiles provide concert histories and map views. Search and filter state is preserved in bookmarkable URLs.

## Technical decisions

- Joins the API's artist, location, date, and relation datasets by artist ID.
- Uses escaped Go templates and embeds templates, styles, map assets, and coordinate data in the executable.
- Validates inputs and handles API failures with timeouts, response limits, and dedicated error pages.
- Converts geographic coordinates into SVG markers using an equirectangular projection.

Maps represent matched locations rather than exact concert venues. Region matching depends on the available geographic metadata.

## Checks

```sh
go vet ./...
go build ./...
```

Manual checks covered search, combined filters, map coverage, error handling, and desktop/mobile layouts with JavaScript disabled. There is no committed automated test suite.

## Author

[Hamza Cheema](https://learn.reboot01.com/git/hcheema)
