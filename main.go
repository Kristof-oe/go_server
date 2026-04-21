package main

import "net/http"

func main() {

	mux := http.NewServeMux()

	mine := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	mine.ListenAndServe()
}
