package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
)

func main() {
	api := NewApi()
	http.HandleFunc("/", api.homeHandler)
	fmt.Printf("About to Listen at port %s", os.Getenv("PORT"))
	err := http.ListenAndServe(fmt.Sprintf(":%s", os.Getenv("PORT")), nil)
	if err != nil {
		panic(err)
	} else {
		fmt.Printf("Listeninng at port %s", os.Getenv("PORT"))
	}

}

func (a *API) homeHandler(w http.ResponseWriter, r *http.Request) {

	q := r.URL.Query().Get("search")

	fmt.Printf("updated home handler. search query is %s\n", q)

	data, cacheHit, err := a.getData(r.Context(), q)

	if err != nil {
		fmt.Printf("error getting api response %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
	}

	resp := ApiResponse{
		Cache: cacheHit,
		Data:  data,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		fmt.Printf("error encoding response %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
	}

}

func (a *API) getData(ctx context.Context, query string) ([]NominatimResponse, bool, error) {

	// is query cached

	escapedQuery := url.PathEscape(query)

	value, err := a.cache.Get(ctx, escapedQuery).Result()

	if err == redis.Nil {
		fmt.Println("key2 does not exist calling external ds")

		address := fmt.Sprintf("https://nominatim.openstreetmap.org/search?q=%s&format=json", escapedQuery)

		resp, err := http.Get(address)
		if err != nil {
			return nil, false, err
		}

		fmt.Printf("response body \n\n%v\n\n", resp.Body)
		data := make([]NominatimResponse, 0)

		err = json.NewDecoder(resp.Body).Decode(&data)

		if err != nil {
			return nil, false, err
		}

		//set value to redis first
		b, err := json.Marshal(data)
		if err != nil {
			fmt.Printf("error occured when marshal json data %v\n\n", err)
			return nil, false, err
		}

		err = a.cache.Set(ctx, escapedQuery, bytes.NewBuffer(b).Bytes(), time.Second*15).Err()

		if err != nil {
			panic(err)
		}

		return data, false, nil

	} else if err != nil {
		fmt.Printf("error calling redis : %v\n\n", err)
		return nil, false, err
	} else {

		fmt.Printf("cache found \n\n")
		data := make([]NominatimResponse, 0)

		err := json.Unmarshal(bytes.NewBufferString(value).Bytes(), &data)
		if err != nil {
			fmt.Printf("error unmarshaling data from  redis cache : %v\n\n", err)
			return nil, false, err
		}

		return data, true, nil

	}

}

type API struct {
	cache *redis.Client
}

func NewApi() *API {
	var opts *redis.Options
	if os.Getenv("LOCAL") == "true" {
		redisAddress := fmt.Sprintf("%s:6379", os.Getenv("REDIS_URL"))
		opts = &redis.Options{
			Addr:     redisAddress,
			Password: "",
			DB:       0,
		}
	} else {
		var host = os.Getenv("REDIS_HOST")
		var password = os.Getenv("REDIS_PASSWORD")
		opts = &redis.Options{
			Addr:      host,
			Password:  password,
			TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		}
	}
	rdb := redis.NewClient(opts)

	ctx := context.Background()
	err := rdb.Ping(ctx).Err()

	if err != nil {
		log.Fatalf("failed to connect with redis instance at- %v\n\n", err)
	}

	return &API{
		cache: rdb,
	}
}

type ApiResponse struct {
	Cache bool                `json:"cache"`
	Data  []NominatimResponse `json: data`
}

type NominatimResponse struct {
	PlaceID     int      `json:"place_id"`
	Licence     string   `json:"licence"`
	OsmType     string   `json:"osm_type"`
	OsmID       int      `json:"osm_id"`
	Boundingbox []string `json:"boundingbox"`
	Lat         string   `json:"lat"`
	Lon         string   `json:"lon"`
	DisplayName string   `json:"display_name"`
	Class       string   `json:"class"`
	Type        string   `json:"type"`
	Importance  float64  `json:"importance"`
	Icon        string   `json:"icon"`
}
