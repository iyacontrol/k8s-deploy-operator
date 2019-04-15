package main

import (
	"fmt"
	"net/http"
)

func sayhelloName(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "hello Release!!")
}

func main() {
	http.HandleFunc("/", sayhelloName)
	http.ListenAndServe(":8080", nil)
}
