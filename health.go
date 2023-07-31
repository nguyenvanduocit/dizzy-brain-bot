package main

import (
	"fmt"
	"net/http"
)

func startHealthServer(httpPort string) error {
	handler := func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "OK")
	}

	// Listen on port 8080.
	http.HandleFunc("/healthz", handler)
	if err := http.ListenAndServe(":"+httpPort, nil); err != nil {
		return err
	}

	return nil
}
