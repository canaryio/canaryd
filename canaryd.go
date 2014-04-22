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

type Measurement struct {
	Id                string  `json:"id"`
	CheckId           string  `json:"check_id"`
	Location          string  `json:"location"`
	Url               string  `json:"url"`
	T                 int     `json:"t"`
	ExitStatus        int     `json:"exit_status"`
	ConnectTime       float64 `json:"connect_time,omitempty"`
	StartTransferTime float64 `json:"starttransfer_time,omitempty"`
	LocalIp           string  `json:"local_ip,omitempty"`
	PrimaryIp         string  `json:"primary_ip,omitempty"`
	TotalTime         float64 `json:"total_time,omitempty"`
	HttpStatus        int     `json:"http_status,omitempty"`
	NameLookupTime    float64 `json:"namelookup_time,omitempty"`
}

func (m *Measurement) Record() {
	s, _ := json.Marshal(m)
	z := redis.Z{Score: float64(m.T), Member: string(s)}
	client.ZAdd(GetRedisKey(m.CheckId), z)
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

		now := time.Now()
		epoch := now.Unix() - 60*60
		client.ZRemRangeByScore("measurements:"+m.CheckId, "-inf", strconv.FormatInt(epoch, 10))
	}

	log.Printf("fn=post_measurements count=%d\n", len(measurements))
}

func ConnectToRedis() {

	u, err := url.Parse(os.Getenv("REDIS_URL"))
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

	r.HandleFunc("/checks", RedirectToChecksHandler)
	r.HandleFunc("/checks/{check_id}/measurements", GetMeasurementsHandler)
	r.HandleFunc("/measurements", PostMeasurementsHandler)
	http.Handle("/", r)

	port := GetenvWithDefault("PORT", "5000")
	log.Printf("fn=main listening=true port=%s\n", port)

	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		panic(err)
	}
}
