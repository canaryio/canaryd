package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/vmihailenco/redis/v2"
)

var client *redis.Client

type measurement struct {
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

func checks(res http.ResponseWriter, req *http.Request) {

}

func get_measurements(res http.ResponseWriter, req *http.Request) {

}

func post_measurements(res http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	measurements := make([]measurement, 0, 100)

	err := decoder.Decode(&measurements)
	if err != nil {
		panic(err)
	}

	for _, m := range measurements {
		s, _ := json.Marshal(m)
		z := redis.Z{Score: float64(m.T), Member: string(s)}
		client.ZAdd("measurements:"+m.CheckId, z)
		now := time.Now()
		epoch := now.Unix() - 60*60
		client.ZRemRangeByScore("measurements:"+m.CheckId, "-inf", string(epoch))
	}

	log.Printf("fn=post_measurements count=%d\n", len(measurements))
}

func connect_to_redis() {
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
	connect_to_redis()

	http.HandleFunc("/checks", checks)
	http.HandleFunc("/measurements", post_measurements)
	http.HandleFunc("/", get_measurements)

	fmt.Println("fn=main listening=true")
	err := http.ListenAndServe(":"+os.Getenv("PORT"), nil)
	if err != nil {
		panic(err)
	}
}
