/*
Copyright 2021 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

func pollHandler(client *http.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("Prober received a request.")

		req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080", nil)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := wait.PollImmediateUntil(20*time.Millisecond, func() (bool, error) {
			res, err := client.Do(req)
			if err != nil {
				log.Println("Do() failed", err)
				return false, nil // Return nil error for retrying.
			}

			// Ensure body is read and closed to ensure connection can be re-used via keep-alive.
			// No point handling errors here, connection just won't be reused.
			defer res.Body.Close()
			defer io.Copy(ioutil.Discard, res.Body)

			log.Println("Prober received a response.", res.StatusCode)

			if res.StatusCode == 200 {
				return true, nil
			}

			// retry
			return false, nil
		}, ctx.Done())

		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func main() {
	log.SetFlags(log.Lmicroseconds)

	ctx, cancel := context.WithCancel(context.Background())
	setupSignals(cancel)

	log.Print("Prober app started.")

	server := &http.Server{
		Addr:    ":8081",
		Handler: pollHandler(httpClient()),
	}

	go server.ListenAndServe()

	<-ctx.Done()

	server.Shutdown(context.TODO())
}

func httpClient() *http.Client {
	return &http.Client{
		Timeout: 1 * time.Second,
	}
}

func setupSignals(cancel func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		cancel()
	}()
}
