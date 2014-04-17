package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

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

func hello(res http.ResponseWriter, req *http.Request) {
	fmt.Fprintln(res, "hello, world")
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

	log.Println(measurements)
}

func main() {
	http.HandleFunc("/", hello)
	http.HandleFunc("/checks", checks)
	http.HandleFunc("/measurements", post_measurements)

	fmt.Println("listening...")
	err := http.ListenAndServe(":"+os.Getenv("PORT"), nil)
	if err != nil {
		panic(err)
	}
}
