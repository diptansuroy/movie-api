package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"net/url"

	"github.com/joho/godotenv"
)

type Movie struct {
	Title        string `json:"Title"`
	Year         string `json:"Year"`
	Plot         string `json:"Plot"`
	Country      string `json:"Country"`
	Awards       string `json:"Awards"`
	Director     string `json:"Director"`
	Genre        string `json:"Genre"`
	Actors       string `json:"Actors"`
	IMDBRating   string `json:"imdbRating"`
	IMDBID       string `json:"imdbID"`
	TotalSeasons string `json:"totalSeasons"`
	Response     string `json:"Response"`
	Error        string `json:"Error"`
}


type Recommendation struct {
	Title      string `json:"Title"`
	Year       string `json:"Year"`
	IMDBRating string `json:"imdbRating"`
	Plot       string `json:"Plot"`
	Reason     string `json:"Reason"`
}
type Season struct {
	Title    string         `json:"Title"`
	Season   string         `json:"Season"`
	Total    string         `json:"totalSeasons"`
	Episodes []EpisodeBrief `json:"Episodes"`
	Response string         `json:"Response"`
	Error    string         `json:"Error"`
}

type EpisodeBrief struct {
	Title string `json:"Title"`
	Released string `json:"Released"`
	Episode string `json:"Episode"`
	IMDBRating string `json:"imdbRating"`
	IMDBID string `json:"imdbID"`
}
var apiKey string

func fetchFromOMDb(query string) (*Movie, error) {
	resp, err := http.Get("http://www.omdbapi.com/?" + query + "&apikey=" + apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to call OMDb API")
	}
	defer resp.Body.Close()

	var movie Movie
	if err := json.NewDecoder(resp.Body).Decode(&movie); err != nil {
		return nil, fmt.Errorf("failed to decode OMDb response")
	}

	if movie.Response == "False" {
		return nil, fmt.Errorf(movie.Error)
	}
	return &movie, nil
}
func fetchSeasonFromOMDb(query string) (*Season, error) {
	resp, err := http.Get("http://www.omdbapi.com/?" + query + "&apikey=" + apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to call OMDb API")
	}
	defer resp.Body.Close()

	var season Season
	if err := json.NewDecoder(resp.Body).Decode(&season); err != nil {
		return nil, fmt.Errorf("failed to decode OMDb response")
	}
	if season.Response == "False" {
		return nil, fmt.Errorf(season.Error)
	}
	return &season, nil
}
func movieDetailsHandler(w http.ResponseWriter, r *http.Request) {
	title := r.URL.Query().Get("title")
	if title == "" {
		http.Error(w, "Missing title query parameter", http.StatusBadRequest)
		return
	}
	movie, err := fetchFromOMDb("t=" + url.QueryEscape(title))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	response := map[string]interface{}{
		"Title":    movie.Title,
		"Year":     movie.Year,
		"Plot":     movie.Plot,
		"Country":  movie.Country,
		"Awards":   movie.Awards,
		"Director": movie.Director,
		"Ratings":  movie.IMDBRating,
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(response)
}

func episodeDetailsHandler(w http.ResponseWriter, r *http.Request) {
	series := r.URL.Query().Get("series_title")
	season := r.URL.Query().Get("season")
	episode := r.URL.Query().Get("episode_number")

	if series == "" || season == "" || episode == "" {
		http.Error(w, "Missing required query parameters", http.StatusBadRequest)
		return
	}

	seriesData, err := fetchFromOMDb("t=" + url.QueryEscape(series))
	if err != nil {
		http.Error(w, "Series not found: "+err.Error(), http.StatusNotFound)
		return
	}

	query := fmt.Sprintf("i=%s&Season=%s&Episode=%s", seriesData.IMDBID, url.QueryEscape(season), url.QueryEscape(episode))
	episodeData, err := fetchFromOMDb(query)
	if err != nil {
		http.Error(w, "Episode not found: "+err.Error(), http.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		"Series":   seriesData.Title,
		"Season":   season,
		"Episode":  episode,
		"Title":    episodeData.Title,
		"Year":     episodeData.Year,
		"Plot":     episodeData.Plot,
		"Director": episodeData.Director,
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(response)
}

func recommendMoviesHandler(w http.ResponseWriter, r *http.Request) {
	fav := r.URL.Query().Get("favorite_movie")
	if fav == "" {
		http.Error(w, "Missing favorite_movie query parameter", http.StatusBadRequest)
		return
	}

	favMovie, err := fetchFromOMDb("t=" + url.QueryEscape(fav))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	titles := getMoviePool()
	recommendations := map[string][]Recommendation{
		"genre_based":    {},
		"director_based": {},
		"actor_based":    {},
	}

	// Global seen map to avoid repeats across all categories
	seenAll := map[string]bool{}

	// GENRE BASED
	favGenres := strings.Split(favMovie.Genre, ", ")
	for _, t := range titles {
		if len(recommendations["genre_based"]) >= 15 {
			break
		}
		m, err := fetchFromOMDb("t=" + url.QueryEscape(t))
		if err == nil && !strings.EqualFold(m.Title, favMovie.Title) {
			if seenAll[m.Title] {
				continue
			}
			for _, g := range favGenres {
				if strings.Contains(strings.ToLower(m.Genre), strings.ToLower(g)) {
					recommendations["genre_based"] = append(recommendations["genre_based"], Recommendation{
						Title:      m.Title,
						Year:       m.Year,
						IMDBRating: m.IMDBRating,
						Plot:       m.Plot,
						Reason:     "Same genre: " + g,
					})
					seenAll[m.Title] = true
					break
				}
			}
		}
	}

	// DIRECTOR BASED
	for _, t := range titles {
		if len(recommendations["director_based"]) >= 15 {
			break
		}
		m, err := fetchFromOMDb("t=" + url.QueryEscape(t))
		if err == nil && !strings.EqualFold(m.Title, favMovie.Title) &&
			strings.EqualFold(strings.TrimSpace(m.Director), strings.TrimSpace(favMovie.Director)) {
			if seenAll[m.Title] {
				continue
			}
			recommendations["director_based"] = append(recommendations["director_based"], Recommendation{
				Title:      m.Title,
				Year:       m.Year,
				IMDBRating: m.IMDBRating,
				Plot:       m.Plot,
				Reason:     "Same director: " + favMovie.Director,
			})
			seenAll[m.Title] = true
		}
	}

	// ACTOR BASED
	favActors := strings.Split(favMovie.Actors, ", ")
	for _, t := range titles {
		if len(recommendations["actor_based"]) >= 15 {
			break
		}
		m, err := fetchFromOMDb("t=" + url.QueryEscape(t))
		if err == nil && !strings.EqualFold(m.Title, favMovie.Title) {
			if seenAll[m.Title] {
				continue
			}
			for _, actor := range favActors {
				actor = strings.TrimSpace(actor)
				if strings.Contains(strings.ToLower(m.Actors), strings.ToLower(actor)) {
					recommendations["actor_based"] = append(recommendations["actor_based"], Recommendation{
						Title:      m.Title,
						Year:       m.Year,
						IMDBRating: m.IMDBRating,
						Plot:       m.Plot,
						Reason:     "Shared actor: " + actor,
					})
					seenAll[m.Title] = true
					break
				}
			}
		}
	}

	// Sort by IMDb rating
	sortByRating := func(movies []Recommendation) {
		sort.Slice(movies, func(i, j int) bool {
			var ri, rj float64
			fmt.Sscanf(movies[i].IMDBRating, "%f", &ri)
			fmt.Sscanf(movies[j].IMDBRating, "%f", &rj)
			return ri > rj
		})
	}
	sortByRating(recommendations["genre_based"])
	sortByRating(recommendations["director_based"])
	sortByRating(recommendations["actor_based"])

	// Trim each to 15 max
	for key, recs := range recommendations {
		if len(recs) > 15 {
			recommendations[key] = recs[:15]
		}
	}

	// Ordered response: genre → director → actor
	resp := map[string]interface{}{
		"favorite_movie": favMovie.Title,
		"recommendations": []map[string]interface{}{
			{"genre_based": recommendations["genre_based"]},
			{"director_based": recommendations["director_based"]},
			{"actor_based": recommendations["actor_based"]},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(resp)
}



func genreTopMoviesHandler(w http.ResponseWriter, r *http.Request) {
	genre := r.URL.Query().Get("genre")
	if genre == "" {
		http.Error(w, "Missing genre query parameter", http.StatusBadRequest)
		return
	}
	titles := getMoviePool()
	var matches []Recommendation
	for _, t := range titles {
		m, err := fetchFromOMDb("t=" + url.QueryEscape(t))
		if err == nil && strings.Contains(strings.ToLower(m.Genre), strings.ToLower(genre)) {
			matches = append(matches, Recommendation{
				Title:      m.Title,
				Year:       m.Year,
				IMDBRating: m.IMDBRating,
				Plot:       m.Plot,
				Reason:     "Genre match: " + genre,
			})
		}
	}
	if len(matches) == 0 {
		http.Error(w, "No movies found for genre "+genre, http.StatusNotFound)
		return
	}
	sort.Slice(matches, func(i, j int) bool {
		var ri, rj float64
		fmt.Sscanf(matches[i].IMDBRating, "%f", &ri)
		fmt.Sscanf(matches[j].IMDBRating, "%f", &rj)
		return ri > rj
	})
	if len(matches) > 15 {
		matches = matches[:15]
	}
	resp := map[string]interface{}{
		"genre":  genre,
		"movies": matches,
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(resp)
}

func seriesDetailsHandler(w http.ResponseWriter, r *http.Request) {
	series := r.URL.Query().Get("title")
	if series == "" {
		http.Error(w, "Missing title query parameter", http.StatusBadRequest)
		return
	}
	seriesData, err := fetchFromOMDb("t=" + url.QueryEscape(series) + "&type=series")
	if err != nil {
		http.Error(w, "Series not found: "+err.Error(), http.StatusNotFound)
		return
	}
	response := map[string]interface{}{
		"Title":        seriesData.Title,
		"Director":     seriesData.Director,
		"Plot":         seriesData.Plot,
		"Total Seasons": seriesData.TotalSeasons,
	}
	writeJSON(w, response)
}


func seasonDetailsHandler(w http.ResponseWriter, r *http.Request) {
	series := r.URL.Query().Get("series_title")
	season := r.URL.Query().Get("season")
	if series == "" || season == "" {
		http.Error(w, "Missing required query parameters", http.StatusBadRequest)
		return
	}

	seriesData, err := fetchFromOMDb("t=" + url.QueryEscape(series) + "&type=series")
	if err != nil {
		http.Error(w, "Series not found: "+err.Error(), http.StatusNotFound)
		return
	}

	seasonData, err := fetchSeasonFromOMDb("i=" + seriesData.IMDBID + "&Season=" + season)
	if err != nil {
		http.Error(w, "Season not found: "+err.Error(), http.StatusNotFound)
		return
	}

	// Build enriched list with plots
	type EpisodeFull struct {
		Title      string `json:"Title"`
		Released   string `json:"Released"`
		Episode    string `json:"Episode"`
		IMDBRating string `json:"imdbRating"`
		IMDBID     string `json:"imdbID"`
		Plot       string `json:"Plot"`
	}
	var enrichedEpisodes []EpisodeFull

	for _, ep := range seasonData.Episodes {
		// Fetch episode details by IMDbID to get the Plot
		details, err := fetchFromOMDb("i=" + ep.IMDBID)
		if err == nil {
			enrichedEpisodes = append(enrichedEpisodes, EpisodeFull{
				Title:      details.Title,
				Released:   ep.Released,
				Episode:    ep.Episode,
				IMDBRating: details.IMDBRating,
				IMDBID:     ep.IMDBID,
				Plot:       details.Plot,
			})
		} else {
			// fallback if plot fetch fails
			enrichedEpisodes = append(enrichedEpisodes, EpisodeFull{
				Title:      ep.Title,
				Released:   ep.Released,
				Episode:    ep.Episode,
				IMDBRating: ep.IMDBRating,
				IMDBID:     ep.IMDBID,
				Plot:       "Plot not available",
			})
		}
	}

	response := map[string]interface{}{
		"Series":   seriesData.Title,
		"Season":   seasonData.Season,
		"Episodes": enrichedEpisodes,
	}
	writeJSON(w, response)
}


func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(data)
}


func getMoviePool() []string {
	return []string{
		"The Shawshank Redemption", "The Godfather", "The Dark Knight", "The Godfather Part II",
		"Pulp Fiction", "Fight Club", "Forrest Gump", "The Lord of the Rings: The Return of the King",
		"The Matrix", "Goodfellas", "Inglourious Basterds", "Interstellar", "The Green Mile",
		"Gladiator", "The Departed", "The Prestige", "Whiplash", "Django Unchained",
		"The Lion King", "Avengers: Endgame", "Memento", "Shutter Island",
		"The Social Network", "Tenet", "Dunkirk", "Catch Me If You Can",
		"Blood Diamond", "The Revenant", "The Wolf of Wall Street", "Titanic",
		"Saving Private Ryan", "Se7en", "The Silence of the Lambs", "The Usual Suspects",
		"Braveheart", "American Beauty", "A Beautiful Mind", "Black Swan",
		"Parasite", "La La Land", "The Big Short", "12 Years a Slave",
		"The Imitation Game", "The Theory of Everything", "No Country for Old Men",
		"There Will Be Blood", "Birdman", "Her", "The Grand Budapest Hotel",
		"Spotlight", "Argo", "The Hurt Locker", "Slumdog Millionaire",
		"Million Dollar Baby", "Mystic River", "The Pianist", "The Truman Show",
		"Eternal Sunshine of the Spotless Mind", "The Sixth Sense", "A Few Good Men",
		"Cast Away", "Apollo 13", "Rain Man", "The Color of Money",
		"Once Upon a Time in Hollywood", "The Irishman", "Marriage Story",
		"Moneyball", "Ocean's Eleven", "Ocean's Twelve", "Ocean's Thirteen",
		"Heat", "Collateral", "Minority Report", "War of the Worlds",
		"Edge of Tomorrow", "Oblivion", "Top Gun: Maverick", "Jerry Maguire",
		"Magnolia", "Boogie Nights", "The Fighter", "American Hustle",
		"Silver Linings Playbook", "Joy", "Unforgiven", "Gran Torino",
		"The Mule", "The Bridges of Madison County", "Batman Begins",
		"The Dark Knight Rises", "Man of Steel", "Justice League",
		"Wonder Woman", "Aquaman", "Black Panther", "Doctor Strange",
		"Iron Man", "Iron Man 2", "Iron Man 3", "Captain America: Civil War",
		"Thor: Ragnarok", "Guardians of the Galaxy", "Guardians of the Galaxy Vol. 2",
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
	}
	apiKey = os.Getenv("OMDB_API_KEY")
	if apiKey == "" {
		fmt.Println("OMDB_API_KEY not set in environment")
		return
	}
	http.HandleFunc("/api/movie", movieDetailsHandler)
	http.HandleFunc("/api/episode", episodeDetailsHandler)
	http.HandleFunc("/api/recommend", recommendMoviesHandler)
	http.HandleFunc("/api/movies/genre", genreTopMoviesHandler)
	http.HandleFunc("/api/series", seriesDetailsHandler)
	http.HandleFunc("/api/season", seasonDetailsHandler)
	fmt.Println("Server running on port 8080")
	http.ListenAndServe(":8080", nil)
}
