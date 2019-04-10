// LLAMA Scraper pulls stats from Collectors and then writes them to the indicated database.
package llama

import (
	"errors"
	"fmt"
	influxdb_client "github.com/influxdata/influxdb1-client/v2"
	"log"
	"sync"
	"time"
)

// Set default timeout for writes to 5 seconds
// This may be worth adding as a parameter in the future
const DefaultTimeout = time.Second * 5

// NewInfluxDbWriter provides a client for writing LLAMA datapoints to InfluxDB
func NewInfluxDbWriter(host string, port string, user string, pass string, db string) (*InfluxDbWriter, error) {
	// Create the InfluxDB writer
	url := fmt.Sprintf("http://%v:%v", host, port)
	log.Println("Creating InfluxDB writer for", url)
	ifdbc, err := influxdb_client.NewHTTPClient(influxdb_client.HTTPConfig{
		Addr:     url,
		Username: user,
		Password: pass,
		Timeout:  DefaultTimeout,
	})
	if err != nil {
		return &InfluxDbWriter{}, err
	}

	writer := &InfluxDbWriter{
		client: ifdbc,
		db:     db,
	}
	return writer, nil
}

// InfluxDbWriter is used for writing datapoints to an InfluxDB instance
type InfluxDbWriter struct {
	client influxdb_client.Client
	db     string
}

// Close will close the InfluxDB client connection and release any associated resources
func (w *InfluxDbWriter) Close() error {
	log.Println("Closing InfluxDB client connection")
	return w.client.Close()
}

// Write will commit the batched points to the database
func (w *InfluxDbWriter) Write(batch influxdb_client.BatchPoints) error {
	// Write to the DB
	start := time.Now()
	err := w.client.Write(batch)
	elapsed := time.Since(start).Seconds()
	if err != nil {
		log.Println("DB write failed after:", elapsed, "seconds")
		return err
	}
	// Only track write delay for successes
	log.Println("DB write completed in:", elapsed, "seconds")
	// TODO(dmar): Log rate of `db_write_delay`
	return nil
}

// Batch will group the points into a batch for writing to the database
func (w *InfluxDbWriter) Batch(points Points) (influxdb_client.BatchPoints, error) {
	// Create batch
	bp, err := influxdb_client.NewBatchPoints(influxdb_client.BatchPointsConfig{
		Database:  w.db,
		Precision: "s", // Write as second precision to improve perf, since we don't need it more granular
	})
	if err != nil {
		return nil, err
	}

	// Convert all points and add to batch
	for _, dp := range points {
		// This is needed because the the InfluxDB client expects the interface type
		// More casting weirdness :/
		// TODO(dmar): Reevaluate this in the future, since this was likely a mistake
		//     initially and isn't necessary.
		newFields := make(map[string]interface{})
		for key, value := range dp.Fields {
			newFields[key] = float64(value)
		}
		pt, err := influxdb_client.NewPoint(
			dp.Measurement,
			dp.Tags,
			newFields,
			dp.Time,
		)
		if err != nil {
			return nil, err
		}
		bp.AddPoint(pt)
	}
	return bp, nil
}

// BatchWrite will group and write the indicates points to the associated InfluxDB host
func (w *InfluxDbWriter) BatchWrite(points Points) error {
	batch, err := w.Batch(points)
	if err != nil {
		return errors.New(fmt.Sprintln("Failed to create batch from points:", err))
	}
	err = w.Write(batch)
	if err != nil {
		return errors.New(fmt.Sprintln("Failed to write batch:", err))
	}
	return nil
}

// Scraper pulls stats from collectors and writes them to a backend
type Scraper struct {
	writer     *InfluxDbWriter
	collectors []Client
	port       string
}

// NewScraper creates and initializes a means of collecting stats and writing them to a database
func NewScraper(collectors []string, cPort string, dbHost string, dbPort string, dbUser string, dbPass string, dbName string) (*Scraper, error) {
	var clients []Client
	for _, collector := range collectors {
		c := NewClient(collector, cPort)
		clients = append(clients, c)
	}
	w, err := NewInfluxDbWriter(dbHost, dbPort, dbUser, dbPass, dbName)
	if err != nil {
		return &Scraper{}, err
	}
	s := &Scraper{
		writer:     w,
		collectors: clients,
		port:       cPort,
	}
	return s, nil
}

// Run performs collections for all assocated collectors
func (s *Scraper) Run() {
	log.Println("Collection cycle starting")
	// Make sure we don't leave DB connections hanging open
	defer s.writer.Close()
	var wg sync.WaitGroup
	// For each collector
	for _, collector := range s.collectors {
		wg.Add(1)
		go func(c Client) {
			defer wg.Done()
			s.run(c)
		}(collector)
	}
	wg.Wait()
	log.Println("Collection cycle complete")
}

func (s *Scraper) run(collector Client) error {
	log.Println(collector.Hostname(), "- Collection cycle started")
	// Pull stats
	points, err := collector.GetPoints()
	numPoints := float64(len(points))
	if err != nil {
		log.Println(collector.Hostname(), "- Collection failed:", err)
		// TODO(dmar): Log rate of `failed_collections`
		return err
	}
	log.Println(collector.Hostname(), "- Pulled datapoints:", numPoints)
	// TODO(dmar): Log rate of `pulled_points`
	// Write them to the client
	err = s.writer.BatchWrite(points)
	if err != nil {
		log.Println(collector.Hostname(), "- Collection failed:", err)
		// TODO(dmar): Log rate of `failed_collections`
		return err
	}
	log.Println(collector.Hostname(), "- Wrote datapoints")
	// TODO(dmar): Log rate of `written_points`
	log.Println(collector.Hostname(), "- Collection cycle completed")
	// TODO(dmar): Log rate of `successful_collections`
	return nil
}
