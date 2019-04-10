// Llama client to pull metrics from Llama collectors
package llama

import (
	"encoding/json"
	"fmt"
	"net/http"
	"io/ioutil"
)

/*
A sample datapoint looks like this:

{
    "fields": {
        "loss": 0.0
        "sent": 480.0,
        "lost": 0.0,
        "rtt": 2.345,
    },
    "tags": {
        "src_metro": "sjc",
        "dst_metro": "dfw",
        "dst_cluster": "dfw3b",
        "dst_host": "dfw3b-rt9-10a",
        "dst_facility": "dfw3",
        "src_cluster": "sjc12b",
        "src_facility": "sjc12",
        "src_host": "sjc12b-ro1-31a"
    },
    "time": 1478807831000000000,
    "measurement": "raw_stats"
}
*/

// TODO(dmar): If this is kept in the future, just use the actual Point
//      from the influxdb library, instead of recreating here and elsewhere
//      within the LLAMA library.
// DataPoint represents an InfluxDB point.
//
// Fields must have `interface{}` values to comply with the Go client
// type DataPoint struct {
// 	Fields      map[string]interface{} `json:"fields"`
// 	Measurement string                 `json:"measurement"`
// 	Tags        map[string]string      `json:"tags"`
// 	Time        time.Time              `json:"time"`
// }

// Points is a collection of DataPoints
type Points []DataPoint

type Getter = func(url string) (resp *http.Response, err error)

// Client is a n interface for pulling stats from LLAMA collectors
type Client interface {
	GetPoints() (Points, error)
	Hostname() string
	Port() string
}

type client struct {
	hostname string
	port     string
	getFunc  Getter
}

// NewClient creates a new collector client with hostname and port
// TODO(dmar): This is likely overkill and should be simplified.
func NewClient(hostname string, port string) *client {
	return &client{hostname: hostname, port: port, getFunc: http.Get}
}

func (c *client) Hostname() string {
	return c.hostname
}

func (c *client) Port() string {
	return c.port
}

// GetPoints will fetch data points from the associated collector
func (c *client) GetPoints() (Points, error) {
	url := fmt.Sprintf("http://%s:%s/influxdata", c.hostname, c.port)

	resp, err := c.getFunc(url)
	if err != nil {
		return Points{}, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Points{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Status: %s (%s)", resp.Status, body)
	}

	var response Points
	err = json.Unmarshal(body, &response)

	if err != nil {
		return Points{}, err
	}

	return response, nil
}
