package main

import (
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/bottlerocketlabs/pair/pkg/handlers"
	"github.com/bottlerocketlabs/pair/pkg/logging"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/ory/graceful"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "80"
	}

	verbose := flag.Bool("v", false, "Verbose logging")
	listenInsecure := flag.String("i", ":"+port, "network address and port to listen on (insecure)")
	flag.Parse()
	logFlags := 0
	logOut := ioutil.Discard
	if *verbose {
		logFlags = log.LstdFlags | log.Lshortfile
		logOut = os.Stderr
	}
	logOut = io.MultiWriter(logOut, logging.NewEUNRLogProcessorWithLicenseKey(os.Getenv("NEW_RELIC_LICENSE_KEY")))
	logger := log.New(logOut, "[server] ", logFlags)

	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName("pair-server-simple"),
		newrelic.ConfigLicense(os.Getenv("NEW_RELIC_LICENSE_KEY")),
		newrelic.ConfigDistributedTracerEnabled(true),
	)
	if err != nil {
		log.Fatalf("could not init new relic")
	}

	s := handlers.NewServer(logger, 120*time.Second)
	mux := http.NewServeMux()
	mux.HandleFunc(newrelic.WrapHandleFunc(app, "/", handlers.Index))
	mux.HandleFunc(newrelic.WrapHandleFunc(app, "/p/", s.BasePipeHandler))
	mux.HandleFunc(newrelic.WrapHandleFunc(app, "/metrics", s.Metrics))

	srvInsecure := graceful.WithDefaults(&http.Server{
		Addr:     *listenInsecure,
		Handler:  s.LogrusLogHandler(mux),
		ErrorLog: logger,
	})

	logger.Println("main: Starting the server")
	if err := graceful.Graceful(srvInsecure.ListenAndServe, srvInsecure.Shutdown); err != nil {
		logger.Fatalln("main: Failed to gracefully shutdown server")
	}
	logger.Println("main: Server was shutdown gracefully")
}
