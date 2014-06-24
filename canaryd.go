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
	"github.com/gorilla/websocket"
	"github.com/rcrowley/go-metrics"
	"github.com/rcrowley/go-metrics/librato"
	"github.com/rcrowley/go-metrics/influxdb"
	"github.com/vmihailenco/msgpack"
	"github.com/vmihailenco/redis/v2"
)

var config Config
var client RedisClient
var wsHub = &WsHub{
	Broadcast: make(chan *Measurement),
	Register: make(chan *WsWrapper),
	UnRegister: make(chan *WsWrapper),
	Connections: make(map[string]map[*WsWrapper]bool),
}

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
	SensordURLs      stringslice
	Port             string
	RedisURL         string
	Retention        int64
	LibratoEmail     string
	LibratoToken     string
	LibratoSource    string
	LogStderr        bool
	InfluxdbHost     string
	InfluxdbDatabase string
	InfluxdbUser     string
	InfluxdbPassword string
}

type RedisClient interface {
	ZAdd(key string, members ...redis.Z) *redis.IntCmd
	ZRemRangeByScore(key, min, max string) *redis.IntCmd
	ZRevRangeByScore(key string, opt redis.ZRangeByScore) *redis.StringSliceCmd
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

type WsHub struct {
	Broadcast         chan *Measurement
	Register          chan *WsWrapper
	UnRegister        chan *WsWrapper
	Connections       map[string]map[*WsWrapper]bool
}

type WsWrapper struct {
	Conn              *websocket.Conn
	CheckID           string
	RemoteAddr        string
}

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Recorder struct {
	client RedisClient
}

func NewRecorder(client RedisClient) *Recorder {
	return &Recorder{client: client}
}

func (self *Recorder) record(measurement *Measurement) {
	s, _ := json.Marshal(measurement)
	key := getRedisKey(measurement.Check.ID)
	
	resp := self.client.ZAdd(key, redis.Z{
		Score: float64(measurement.T),
		Member: string(s),
	})
	
	if resp.Err() != nil {
		log.Fatalf("Error while recording measurement %s: %v\n", measurement.ID, resp.Err())
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

func httpServer(config Config, hub *WsHub) {
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
	router.HandleFunc("/ws/checks/{check_id}/measurements", func(res http.ResponseWriter, req *http.Request){
		websocketHandler(hub, res, req)
	});
	http.Handle("/", router)

	log.Printf("fn=httpServer listening=true port=%s\n", config.Port)

	err := http.ListenAndServe(":"+config.Port, nil)
	if err != nil {
		panic(err)
	}
}

func udpServer(port string, toRecorder chan Measurement, toWebsocket chan Measurement) {
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
		toWebsocket <- m
	}
}

func websocketHandler(hub *WsHub, res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	checkID := vars["check_id"]

	conn, err := wsUpgrader.Upgrade(res, req, nil)
	if err != nil {
		log.Printf("fn=websocketHandler remoteAddr=%s checkID=%s connectWsClient=false err=%v\n", req.RemoteAddr, checkID, err)
		return
	}

	defer conn.Close()

	wsWrapper := WsWrapper{
		Conn: conn,
		CheckID: checkID,
		RemoteAddr: req.RemoteAddr,
	}

	hub.Register <- &wsWrapper

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			//remove websocket from subscribe list
			hub.UnRegister <- &wsWrapper
			break
		}
	}
}

func websocketWriter(config Config, hub *WsHub, toWebsocket chan Measurement) {
	for m := range toWebsocket {
		hub.Broadcast <- &m
	}
}

func runWebsocketHub(hub *WsHub) {
	for {
		select {
			case wsWrapper := <-hub.Register:
				checkMap := hub.Connections[wsWrapper.CheckID]
				if checkMap == nil {
					hub.Connections[wsWrapper.CheckID] = make(map[*WsWrapper]bool)
				}
				hub.Connections[wsWrapper.CheckID][wsWrapper] = true
				log.Printf("fn=runWebsocketHub#Register remoteAddr=%s checkID=%s connectWsClient=true\n", wsWrapper.RemoteAddr, wsWrapper.CheckID)
			case wsWrapper := <-hub.UnRegister:
				log.Printf("fn=runWebsocketHub#UnRegister remoteAddr=%s checkID=%s removeWsClient=true \n", wsWrapper.RemoteAddr, wsWrapper.CheckID)
				delete(hub.Connections[wsWrapper.CheckID], wsWrapper)
			case m := <-hub.Broadcast:
				for wsWrapper := range hub.Connections[m.Check.ID] {
					err := wsWrapper.Conn.WriteJSON(m)
					if err != nil {
						delete(hub.Connections[m.Check.ID], wsWrapper)
						log.Printf("fn=runWebsocketHub#Broadcast remoteAddr=%s checkID=%s removeWsClient=true \n", wsWrapper.RemoteAddr, wsWrapper.CheckID)
					}
				}
		}
	}
}

func recordMeasurements(config Config, recorder *Recorder, toRecorder chan Measurement) {
	recordTimer := metrics.NewTimer()
	metrics.Register("canaryd.record", recordTimer)

	trimTimer := metrics.NewTimer()
	metrics.Register("canaryd.trim", trimTimer)

	for {
		m := <-toRecorder
		recordTimer.Time(func() { recorder.record(&m) })
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

	config.InfluxdbHost     = os.Getenv("INFLUXDB_HOST")
	config.InfluxdbDatabase = os.Getenv("INFLUXDB_DATABASE")
	config.InfluxdbUser     = os.Getenv("INFLUXDB_USER")
	config.InfluxdbPassword = os.Getenv("INFLUXDB_PASSWORD")
}

func main() {
	toRecorder := make(chan Measurement)
	toWebsocket := make(chan Measurement)

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

	if config.InfluxdbHost != "" &&
	   config.InfluxdbDatabase != "" &&
	   config.InfluxdbUser != "" &&
	   config.InfluxdbPassword != "" {
		log.Println("fn=main metrics=influxdb")

		go influxdb.Influxdb(metrics.DefaultRegistry, 10e9, &influxdb.Config{
			Host:     config.InfluxdbHost,
			Database: config.InfluxdbDatabase,
			Username: config.InfluxdbUser,
			Password: config.InfluxdbPassword,
		})
	}

	connectToRedis(config)

	go runWebsocketHub(wsHub)
	go httpServer(config, wsHub)
	go udpServer(config.Port, toRecorder, toWebsocket)
	go websocketWriter(config, wsHub, toWebsocket)

	recorder := NewRecorder(client)
	go recordMeasurements(config, recorder, toRecorder)

	select {}
}
