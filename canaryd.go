package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/rcrowley/go-metrics"
	"github.com/rcrowley/go-metrics/librato"
	"github.com/vmihailenco/msgpack"
	"github.com/vmihailenco/redis/v2"
)

var config Config
var client *redis.Client

type stringslice []string

func (s *stringslice) String() string {
	return fmt.Sprint(*s)
}

func (s *stringslice) Set(value string) error {
	for _, ss := range strings.Split(value, ",") {
		*s = append(*s, ss)
	}
	return nil
}

type Config struct {
	SensordURLs   stringslice
	Port          string
	RedisURL      string
	Retention     int64
	LibratoEmail  string
	LibratoToken  string
	LibratoSource string
	LogStderr     bool
}

type Check struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type Measurement struct {
	Check             Check   `json:"check"`
	ID                string  `json:"id"`
	Location          string  `json:"location"`
	T                 int     `json:"t"`
	ExitStatus        int     `json:"exit_status"`
	HTTPStatus        int     `json:"http_status,omitempty"`
	LocalIP           string  `json:"local_ip,omitempty"`
	PrimaryIP         string  `json:"primary_ip,omitempty"`
	NameLookupTime    float64 `json:"namelookup_time,omitempty"`
	ConnectTime       float64 `json:"connect_time,omitempty"`
	StartTransferTime float64 `json:"starttransfer_time,omitempty"`
	TotalTime         float64 `json:"total_time,omitempty"`
	SizeDownload      float64 `json:"size_download,omitempty"`
}

func (m *Measurement) record() {
	s, _ := json.Marshal(m)
	z := redis.Z{Score: float64(m.T), Member: string(s)}
	r := client.ZAdd(getRedisKey(m.Check.ID), z)
	if r.Err() != nil {
		log.Fatalf("Error while recording measurement %s: %v\n", m.ID, r.Err())
	}
}

func trimMeasurements(checkID string, seconds int64) {
	now := time.Now()
	epoch := now.Unix() - seconds
	r := client.ZRemRangeByScore(getRedisKey(checkID), "-inf", strconv.FormatInt(epoch, 10))
	if r.Err() != nil {
		log.Fatalf("Error while trimming check_id %s: %v\n", checkID, r.Err())
	}
}

func getRedisKey(checkID string) string {
	return "measurements:" + checkID
}

func getMeasurementsByRange(checkID string, r int64) []Measurement {
	now := time.Now()
	from := now.Unix() - r

	return getMeasurementsFrom(checkID, from)
}

func getMeasurementsFrom(checkID string, from int64) []Measurement {
	vals, err := client.ZRevRangeByScore(getRedisKey(checkID), redis.ZRangeByScore{
		Min: strconv.FormatInt(from, 10),
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

func getFormValueWithDefault(req *http.Request, key string, def string) string {
	s := req.FormValue(key)
	if s != "" {
		return s
	}
	return def
}

func connectToRedis(config Config) {
	u, err := url.Parse(config.RedisURL)
	if err != nil {
		panic(err)
	}

	client = redis.NewTCPClient(&redis.Options{
		Addr:     u.Host,
		Password: "", // no password set
		DB:       0,  // use default DB
	})
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK\n")
}

func httpServer(config Config) {
	timer := metrics.NewTimer()
	metrics.Register("canaryd.get_measurements", timer)

	measurementsReqHandler := func(res http.ResponseWriter, req *http.Request) {
		timer.Time(func() {
			vars := mux.Vars(req)
			checkID := vars["check_id"]
			rS := getFormValueWithDefault(req, "range", "10")

			r, err := strconv.ParseInt(rS, 10, 64)
			if err != nil {
				panic(nil)
			}
			log.Printf("fn=getMeasurements ip=%s range=%d\n", req.RemoteAddr, r)
			res.Header().Set("Access-Control-Allow-Origin", "*")
			res.Header().Set("Access-Control-Allow-Methods", "GET")
			res.Header().Set("Content-Type", "application/json")
			json.NewEncoder(res).Encode(getMeasurementsByRange(checkID, r))
		})
	}

	router := mux.NewRouter()
	router.HandleFunc("/health", healthHandler).Methods("GET")
	router.HandleFunc("/checks/{check_id}/measurements", measurementsReqHandler).Methods("GET")
	http.Handle("/", router)

	log.Printf("fn=httpServer listening=true port=%s\n", config.Port)

	err := http.ListenAndServe(":"+config.Port, nil)
	if err != nil {
		panic(err)
	}
}

func udpServer(port string, toRecorder chan Measurement) {
	udpAddr, err := net.ResolveUDPAddr("udp4", "0.0.0.0:"+port)
	if err != nil {
		log.Fatal(err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("fn=udpServer listening=true port=%s\n", config.Port)

	for {
		var buf [512]byte
		n, _, err := conn.ReadFromUDP(buf[0:])
		if err != nil {
			log.Fatal(err)
		}

		payload := buf[0:n]
		var m Measurement
		err = msgpack.Unmarshal(payload, &m)
		if err != nil {
			log.Fatal(err)
		}

		toRecorder <- m
	}
}

func recorder(config Config, toRecorder chan Measurement) {
	recordTimer := metrics.NewTimer()
	metrics.Register("canaryd.record", recordTimer)

	trimTimer := metrics.NewTimer()
	metrics.Register("canaryd.trim", trimTimer)

	for {
		m := <-toRecorder
		recordTimer.Time(func() { m.record() })
		trimTimer.Time(func() { trimMeasurements(m.Check.ID, config.Retention) })
	}
}

func getEnvWithDefault(name, def string) string {
	val := os.Getenv(name)
	if val != "" {
		return val
	}

	return def
}

func init() {
	config.Port = getEnvWithDefault("PORT", "5000")
	config.RedisURL = getEnvWithDefault("REDIS_URL", "redis://localhost:6379")

	retention, err := strconv.ParseInt(getEnvWithDefault("RETENTION", "60"), 10, 64)
	if err != nil {
		log.Fatal(err)
	}
	config.Retention = retention

	config.LibratoEmail = os.Getenv("LIBRATO_EMAIL")
	config.LibratoToken = os.Getenv("LIBRATO_TOKEN")
	if os.Getenv("LIBRATO_SOURCE") != "" {
		config.LibratoSource = os.Getenv("LIBRATO_SOURCE")
	} else {
		hostname, err := os.Hostname()
		if err != nil {
			log.Fatal(err)
		}
		config.LibratoSource = hostname
	}
	
	if os.Getenv("LOGSTDERR") == "1" {
		config.LogStderr = true
	}
}

func main() {
	toRecorder := make(chan Measurement)

	if config.LibratoEmail != "" && config.LibratoToken != "" && config.LibratoSource != "" {
		log.Println("fn=main metrics=librato")
		go librato.Librato(metrics.DefaultRegistry,
			10e9,                  // interval
			config.LibratoEmail,   // account owner email address
			config.LibratoToken,   // Librato API token
			config.LibratoSource,  // source
			[]float64{50, 95, 99}, // precentiles to send
			time.Millisecond,      // time unit
		)
	}

	if config.LogStderr == true {
		go metrics.Log(metrics.DefaultRegistry, 10e9, log.New(os.Stderr, "metrics: ", log.Lmicroseconds))
	}

	connectToRedis(config)

	go httpServer(config)
	go udpServer(config.Port, toRecorder)
	go recorder(config, toRecorder)

	select {}
}
