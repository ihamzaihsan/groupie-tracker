package main

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type filterOption struct {
	Value    string
	Label    string
	Selected bool
}

type locationRegionGroup struct {
	Region filterOption
	Cities []filterOption
	Active bool
}

type locationFilterGroup struct {
	Country filterOption
	Regions []locationRegionGroup
	Cities  []filterOption
	Active  bool
}

type artistFilters struct {
	CreationFrom      string
	CreationTo        string
	AlbumFrom         string
	AlbumTo           string
	Members           []filterOption
	Locations         []locationFilterGroup
	Active            bool
	SelectedLocations int
	ResetURL          string
	ClearSearchURL    string
	creationFrom      int
	creationTo        int
	albumFrom         time.Time
	albumTo           time.Time
	members           map[int]bool
	locations         map[string]bool
}

func parseArtistFilters(params url.Values) (artistFilters, error) {
	f := artistFilters{members: make(map[int]bool), locations: make(map[string]bool)}
	for _, key := range []string{"creation_from", "creation_to", "album_from", "album_to"} {
		if len(params[key]) > 1 {
			return f, fmt.Errorf("Supply one value for each date boundary")
		}
	}
	f.CreationFrom, f.CreationTo = params.Get("creation_from"), params.Get("creation_to")
	f.AlbumFrom, f.AlbumTo = params.Get("album_from"), params.Get("album_to")
	for _, bound := range []struct {
		text   string
		target *int
	}{{f.CreationFrom, &f.creationFrom}, {f.CreationTo, &f.creationTo}} {
		if bound.text == "" {
			continue
		}
		year, err := strconv.Atoi(bound.text)
		if err != nil || year < 1 || year > 9999 || strconv.Itoa(year) != bound.text {
			return f, fmt.Errorf("Formation years must be whole numbers from 1 to 9999")
		}
		*bound.target = year
		f.Active = true
	}
	for _, bound := range []struct {
		text   string
		target *time.Time
	}{{f.AlbumFrom, &f.albumFrom}, {f.AlbumTo, &f.albumTo}} {
		if bound.text == "" {
			continue
		}
		date, err := time.Parse("2006-01-02", bound.text)
		if err != nil || date.Year() < 1 {
			return f, fmt.Errorf("Album boundaries must be valid dates in YYYY-MM-DD format")
		}
		*bound.target = date
		f.Active = true
	}
	if (f.creationFrom > 0 && f.creationTo > 0 && f.creationFrom > f.creationTo) || (!f.albumFrom.IsZero() && !f.albumTo.IsZero() && f.albumFrom.After(f.albumTo)) {
		return f, fmt.Errorf("The start of a date range must not be after its end")
	}
	if len(params["members"]) > 100 || len(params["location"]) > 500 {
		return f, fmt.Errorf("Too many filter selections")
	}
	for _, value := range params["members"] {
		count, err := strconv.Atoi(value)
		if err != nil || count < 1 || count > 100 || strconv.Itoa(count) != value {
			return f, fmt.Errorf("Choose a member count from the collection")
		}
		f.members[count] = true
	}
	for _, value := range params["location"] {
		f.locations[value] = true
	}
	f.Active = f.Active || len(f.members) > 0 || len(f.locations) > 0
	f.SelectedLocations = len(f.locations)
	reset := url.Values{}
	for _, key := range []string{"q", "sort", "view"} {
		if value := params.Get(key); value != "" {
			reset.Set(key, value)
		}
	}
	f.ResetURL = "/?" + reset.Encode() + "#collection"
	clear := url.Values{}
	for key, values := range params {
		if key != "q" {
			clear[key] = append([]string(nil), values...)
		}
	}
	f.ClearSearchURL = "/?" + clear.Encode() + "#collection"
	return f, nil
}

// Country and region options contain their cities. Raw API country labels are
// preserved; region metadata comes from the reviewed geocoding snapshot.
func locationFilterTokens(key string, catalog map[string]geographicPoint) (country, region string) {
	parts := strings.SplitN(key, "-", 2)
	if len(parts) != 2 {
		return "", ""
	}
	country = parts[1]
	if point, ok := catalog[key]; ok && point.Note == "" {
		matched := strings.Split(point.MatchedPlace, ", ")
		if len(matched) == 3 {
			region = matched[1]
		}
	}
	return country, region
}

func (f *artistFilters) buildOptions(artists []Artist, locationsByID map[int][]string, catalog map[string]geographicPoint) error {
	counts := make(map[int]bool)
	groups := make(map[string]*locationFilterGroup)
	options := make(map[string]bool)
	cities := make(map[string]bool)
	regionIndexes := make(map[string]int)
	for _, artist := range artists {
		counts[len(artist.Members)] = true
		for _, key := range locationsByID[artist.ID] {
			country, region := locationFilterTokens(key, catalog)
			if country == "" {
				continue
			}
			countryValue := "country:" + country
			if groups[country] == nil {
				groups[country] = &locationFilterGroup{Country: filterOption{countryValue, formatLocation(country), f.locations[countryValue]}}
			}
			options[countryValue] = true
			cityValue := "city:" + key
			if cities[cityValue] {
				continue
			}
			city := filterOption{cityValue, formatLocation(strings.SplitN(key, "-", 2)[0]), f.locations[cityValue]}
			cities[cityValue], options[cityValue] = true, true
			if region == "" {
				groups[country].Cities = append(groups[country].Cities, city)
				continue
			}
			value := "region:" + country + "|" + region
			index, exists := regionIndexes[value]
			if !exists {
				index = len(groups[country].Regions)
				regionIndexes[value] = index
				groups[country].Regions = append(groups[country].Regions, locationRegionGroup{Region: filterOption{value, region, f.locations[value]}})
				options[value] = true
			}
			groups[country].Regions[index].Cities = append(groups[country].Regions[index].Cities, city)
		}
	}
	for count := range f.members {
		if !counts[count] {
			return fmt.Errorf("Choose a member count from the collection")
		}
	}
	for value := range f.locations {
		if !options[value] {
			return fmt.Errorf("Choose a concert location from the collection")
		}
	}
	for count := range counts {
		if count > 0 {
			f.Members = append(f.Members, filterOption{strconv.Itoa(count), strconv.Itoa(count), f.members[count]})
		}
	}
	sort.Slice(f.Members, func(i, j int) bool {
		a, _ := strconv.Atoi(f.Members[i].Value)
		b, _ := strconv.Atoi(f.Members[j].Value)
		return a < b
	})
	for _, group := range groups {
		group.Active = group.Country.Selected
		for _, city := range group.Cities {
			group.Active = group.Active || city.Selected
		}
		for i := range group.Regions {
			region := &group.Regions[i]
			region.Active = region.Region.Selected
			for _, city := range region.Cities {
				region.Active = region.Active || city.Selected
			}
			group.Active = group.Active || region.Active
			sort.Slice(region.Cities, func(i, j int) bool { return region.Cities[i].Label < region.Cities[j].Label })
		}
		sort.Slice(group.Cities, func(i, j int) bool { return group.Cities[i].Label < group.Cities[j].Label })
		sort.Slice(group.Regions, func(i, j int) bool { return group.Regions[i].Region.Label < group.Regions[j].Region.Label })
		f.Locations = append(f.Locations, *group)
	}
	sort.Slice(f.Locations, func(i, j int) bool { return f.Locations[i].Country.Label < f.Locations[j].Country.Label })
	return nil
}

func (f artistFilters) matches(artist Artist, locations []string, catalog map[string]geographicPoint) bool {
	if (f.creationFrom > 0 && artist.CreationDate < f.creationFrom) || (f.creationTo > 0 && artist.CreationDate > f.creationTo) {
		return false
	}
	if !f.albumFrom.IsZero() || !f.albumTo.IsZero() {
		date, err := time.Parse("02-01-2006", strings.TrimPrefix(artist.FirstAlbum, "*"))
		if err != nil || (!f.albumFrom.IsZero() && date.Before(f.albumFrom)) || (!f.albumTo.IsZero() && date.After(f.albumTo)) {
			return false
		}
	}
	if len(f.members) > 0 && !f.members[len(artist.Members)] {
		return false
	}
	if len(f.locations) > 0 {
		for _, key := range locations {
			country, region := locationFilterTokens(key, catalog)
			if f.locations["city:"+key] || f.locations["country:"+country] || (region != "" && f.locations["region:"+country+"|"+region]) {
				return true
			}
		}
		return false
	}
	return true
}
