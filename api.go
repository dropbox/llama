package llama

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
)

// API represnts the HTTP server answering queries for collected data.
type API struct {
	summarizer *Summarizer
	server     *http.Server
	ts         TagSet
	handler    *http.ServeMux
	mutex      sync.RWMutex
}

// InfluxHandler handles requests for InfluxDB formatted summaries.
func (api *API) InfluxHandler(rw http.ResponseWriter, request *http.Request) {
	// Lock the existing summaries cache
	api.summarizer.CMutex.RLock()
	summaries := api.summarizer.Cache
	log.Println("Found", len(summaries), "data points")
	// Convert the summaries to influx datapoints
	api.mutex.RLock()
	ifdp := NewFromSummaries(summaries, api.ts)
	api.mutex.RUnlock()
	// And unlock the cache
	api.summarizer.CMutex.RUnlock()

	// Convert to JSON
	asJson, err := json.Marshal(ifdp)
	if err != nil {
		log.Println(err)
		rw.WriteHeader(500)
		return
	}

	// Send back the response
	rw.Write(asJson)
}

// StatusHandler acts as a back healthcheck and simply returns 200 OK.
func (api *API) StatusHandler(rw http.ResponseWriter, request *http.Request) {
	fmt.Fprintf(rw, "ok")
}

// Stop will close down the server and cause Run to exit.
func (api *API) Stop() {
	err := api.server.Close()
	if err != nil {
		log.Println("Error stopping API:", err)
	}
	log.Println("API Stopped")
}

// Run calls RunForever in a separate goroutine for non-blocking behavior.
func (api *API) Run() {
	// This basically just exists to be consistent with the existing pattern
	// while also allowing it to be run blocking if desired.
	go api.RunForever()
}

// MergeUpdateTagSet combines a provided TagSet with the existing one
func (api *API) MergeUpdateTagSet(t TagSet) {
	api.mutex.Lock()
	// Copy new entries into the existing TagSet
	// Allowing retention of existing entries, updating where needed, and adding new
	for k, v := range t {
		api.ts[k] = v
	}
	api.mutex.Unlock()
}

// RunForever sets up the handlers above and then listens for requests until
// stopped or a fatal error occurs.
//
// Calling this will block until stopped/crashed.
func (api *API) RunForever() {
	// Setup the handlers
	// TODO(dmar): It might be better to move this elsewhere?
	api.setupHandlers()
	// TODO(dmar): Better handling around if this dies or gets shutdown. Though
	//      if it dies, the collector is kinda useless anyways.
	log.Fatal(api.server.ListenAndServe())
}

// SetupHandlers attaches the handlers above to the http server mux.
func (api *API) setupHandlers() {
	api.handler.HandleFunc("/status", api.StatusHandler)
	api.handler.HandleFunc("/influxdata", api.InfluxHandler)
}

// New returns an initialized API struct.
func NewAPI(s *Summarizer, t TagSet, addr string) *API {
	// TODO(dmar): In the future, make these options that can be provided.
	handler := http.NewServeMux()
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}
	return &API{summarizer: s, ts: t, handler: handler, server: server}
}
