package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

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

func post_measurements(res http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	var measurements []measurement
	err := decoder.Decode(&measurements)
	if err != nil {
		panic(err)
	}

	for _, m := range measurements {
		s, _ := json.Marshal(m)

		z := redis.Z{Score: float64(m.T), Member: string(s)}
		client.ZAdd("measurements:"+m.CheckId, z)

		log.Println(string(s))
	}
}

func main() {
	client = redis.NewTCPClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	defer client.Close()

	http.HandleFunc("/checks", checks)
	http.HandleFunc("/measurements", post_measurements)

	fmt.Println("listening...")
	err := http.ListenAndServe(":"+os.Getenv("PORT"), nil)
	if err != nil {
		panic(err)
	}
}
