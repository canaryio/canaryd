package main

import (
	"fmt"
	"net/http"
	"os"
)

func hello(res http.ResponseWriter, req *http.Request) {
	fmt.Fprintln(res, "hello, world")
}

func checks(res http.ResponseWriter, req *http.Request) {

}

func measurements(res http.ResponseWriter, req *http.Request) {

}

func main() {
	http.HandleFunc("/", hello)
	http.HandleFunc("/checks", checks)

	fmt.Println("listening...")
	err := http.ListenAndServe(":"+os.Getenv("PORT"), nil)
	if err != nil {
		panic(err)
	}
}
