// LLAMA Scraper pulls stats from Collectors and then writes them to the indicated database.
package main

import (
	"flag"
	"github.com/dropbox/go-llama"
	"log"
	"strings"
	"time"
)

// Flags
var interval = flag.Int64("llama.interval", 30, "How often to pull stats from collectors, in seconds")
var influxdbHost = flag.String("llama.influxdb-host", "127.0.0.1", "The ip of the host running the InfluxDB server")
var influxdbPort = flag.String("llama.influxdb-port", "5086", "The port the InfluxDB server is listening on")
var influxdbDb = flag.String("llama.influxdb-name", "llama", "The InfluxDB database name")
var collectorPort = flag.String("llama.collector-port", "5000", "The port collectors are listening on")
var collectorHosts = flag.String("llama.collector-hosts", "", "Comma-separated list of hostnames/IP addresses for collectors")
var influxdbUser = flag.String("llama.influxdb-user", "", "The name of the user to use with InfluxDB")
var influxdbPass = flag.String("llama.influxdb-pass", "", "The password to use with InfluxDB")

func main() {
	flag.Parse()

	// Make sure we have some collectors
	collectors := strings.Split(*collectorHosts, ",")
	if len(collectors) < 1 {
		log.Fatal("No collectors provided; aborting")
	}

	scraper, err := llama.NewScraper(collectors, *collectorPort, *influxdbHost, *influxdbPort, *influxdbUser, *influxdbPass, *influxdbDb)
	if err != nil {
		log.Fatalln("Unable to create scraper: ", err)
	}

	// Setup a timer, and perform collections each tick
	log.Println("Starting ticker for collection every", *interval, "seconds")
	for now := range time.Tick(time.Duration(*interval) * time.Second) {
		log.Println("Starting collection at tick:", now)
		scraper.Run()
	}
}
