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
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

var gate int32 = 0

func handler(w http.ResponseWriter, r *http.Request) {
	if atomic.LoadInt32(&gate) != 1 {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log.Println("Hello world received a request.")
	fmt.Fprintln(w, "Hello World! How about some tasty noodles? ", time.Now().UTC().Format(time.StampMilli))
}

func main() {
	log.SetFlags(log.Lmicroseconds)

	ctx, cancel := context.WithCancel(context.Background())
	setupSignals(cancel)

	log.Print("Hello world app started.")
	setupDelay(ctx)

	server := &http.Server{Addr: ":8080", Handler: http.HandlerFunc(handler)}

	go server.ListenAndServe()

	<-ctx.Done()

	server.Shutdown(context.TODO())
}

func setupDelay(ctx context.Context) {
	go func() {
		sleep, _ := time.ParseDuration(os.Getenv("START_DELAY"))
		if sleep == 0 {
			sleep = 500 * time.Millisecond
		}

		log.Println("sleeping for", sleep.String())
		select {
		case <-ctx.Done():
			return
		case <-time.After(sleep):
		}

		log.Println("sleeping done")
		atomic.StoreInt32(&gate, 1)
	}()
}

func setupSignals(cancel func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		cancel()
	}()
}
