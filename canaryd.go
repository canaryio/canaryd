package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/vmihailenco/redis/v2"
)

var client *redis.Client

type Check struct {
	Id  string `json:"id"`
	Url string `json:"url"`
}

type Measurement struct {
	Check             Check   `json:"check"`
	Id                string  `json:"id"`
	Location          string  `json:"location"`
	T                 int     `json:"t"`
	ExitStatus        int     `json:"exit_status"`
	HttpStatus        int     `json:"http_status,omitempty"`
	LocalIp           string  `json:"local_ip,omitempty"`
	PrimaryIp         string  `json:"primary_ip,omitempty"`
	NameLookupTime    float64 `json:"namelookup_time,omitempty"`
	ConnectTime       float64 `json:"connect_time,omitempty"`
	StartTransferTime float64 `json:"starttransfer_time,omitempty"`
	TotalTime         float64 `json:"total_time,omitempty"`
}

func (m *Measurement) Record() {
	s, _ := json.Marshal(m)
	z := redis.Z{Score: float64(m.T), Member: string(s)}
	r := client.ZAdd(GetRedisKey(m.Check.Id), z)
	if r.Err() != nil {
		log.Fatalf("Error while recording measuremnt %s: %v\n", m.Id, r.Err())
	}
}

func TrimMeasurements(check_id string, seconds int64) {
	now := time.Now()
	epoch := now.Unix() - seconds
	r := client.ZRemRangeByScore(GetRedisKey(check_id), "-inf", strconv.FormatInt(epoch, 10))
	if r.Err() != nil {
		log.Fatalf("Error while trimming check_id %s: %v\n", check_id, r.Err())
	}
}

func GetRedisKey(check_id string) string {
	return "measurements:" + check_id
}

func GetLatestMeasurements(check_id string) []Measurement {
	now := time.Now()
	epoch := now.Unix() - 60

	vals, err := client.ZRevRangeByScore(GetRedisKey(check_id), redis.ZRangeByScore{
		Min: strconv.FormatInt(epoch, 10),
		Max: "+inf",
	}).Result()

	if err != nil {
		panic(err)
	}

	measurements := make([]Measurement, 0, 100)

	for _, v := range vals {
		var m Measurement
		json.Unmarshal([]byte(v), &m)
		measurements = append(measurements, m)
	}

	return measurements
}

func GetenvWithDefault(key string, def string) string {
	try := os.Getenv(key)

	if try == "" {
		return def
	}

	return try
}

func RedirectToChecksHandler(res http.ResponseWriter, req *http.Request) {
	checks_url := GetenvWithDefault("CHECKS_URL", "https://s3.amazonaws.com/canary-public-data/data.json")
	http.Redirect(res, req, checks_url, http.StatusFound)
}

func GetMeasurementsHandler(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	check_id := vars["check_id"]

	s, _ := json.MarshalIndent(GetLatestMeasurements(check_id), "", "  ")

	fmt.Fprintf(res, string(s))
}

func PostMeasurementsHandler(res http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	measurements := make([]Measurement, 0, 100)

	err := decoder.Decode(&measurements)
	if err != nil {
		panic(err)
	}

	for _, m := range measurements {
		m.Record()
		TrimMeasurements(m.Check.Id, 60*60)
	}

	log.Printf("fn=post_measurements count=%d\n", len(measurements))
}

func ConnectToRedis() {
	u, err := url.Parse(GetenvWithDefault("REDIS_URL", "redis://localhost:6379"))
	if err != nil {
		panic(err)
	}

	client = redis.NewTCPClient(&redis.Options{
		Addr:     u.Host,
		Password: "", // no password set
		DB:       0,  // use default DB
	})
}

func main() {
	ConnectToRedis()

	r := mux.NewRouter()

	r.HandleFunc("/checks", RedirectToChecksHandler).Methods("GET")
	r.HandleFunc("/checks/{check_id}/measurements", GetMeasurementsHandler).Methods("GET")
	r.HandleFunc("/measurements", PostMeasurementsHandler).Methods("POST")
	http.Handle("/", r)

	port := GetenvWithDefault("PORT", "5000")
	log.Printf("fn=main listening=true port=%s\n", port)

	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		panic(err)
	}
}
