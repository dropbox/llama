package llama

import (
	influxdb_client "github.com/influxdata/influxdb1-client/v2"
	gocheck "gopkg.in/check.v1"
	"testing"
	"time"
)

type MockIfdbClient struct {
	influxdb_client.Client
}

func (m *MockIfdbClient) Ping(timeout time.Duration) (time.Duration, string, error) {
	return time.Second, "", nil
}

func (m *MockIfdbClient) Write(bp influxdb_client.BatchPoints) error {
	return nil
}

func (m *MockIfdbClient) Query(q influxdb_client.Query) (*influxdb_client.Response, error) {
	return &influxdb_client.Response{}, nil
}

func (m *MockIfdbClient) Close() error {
	return nil
}

// Setup testing environment
func TestCheckSetup(t *testing.T) { gocheck.TestingT(t) }

type ScraperSuite struct {
	ifdbc   influxdb_client.Client
	writer  *InfluxDbWriter
	scraper *Scraper
}

var _ = gocheck.Suite(&ScraperSuite{})

// Perform per test
func (s *ScraperSuite) SetUpTest(c *gocheck.C) {
}

func (s *ScraperSuite) TearDownTest(c *gocheck.C) {
}

// Performed for whole suite
func (s *ScraperSuite) SetUpSuite(c *gocheck.C) {
	// Mock version of the underlying InfluxDB client
	s.ifdbc = &MockIfdbClient{}
	// Mock collector clients
	collectors := []Client{
		&MockClient{},
		&MockClient{},
	}
	// Mock version of the LLAMA influxdb connector
	s.writer = &InfluxDbWriter{
		client: s.ifdbc,
		db:     "dbname",
	}
	// Mock scraper
	s.scraper = &Scraper{
		writer:     s.writer,
		collectors: collectors,
		port:       "5000",
	}
}

func (s *ScraperSuite) TearDownSuite(c *gocheck.C) {
}

var examplePoints = Points{
	DataPoint{
		Fields: map[string]IDBFloat64{
			"loss": 0.0,
			"sent": 480.0,
			"lost": 0.0,
			"rtt":  2.45,
		},
		Measurement: "measurement",
		Tags: map[string]string{
			"src_metro": "abc",
			"dst_metro": "xyz",
		},
		// 2018-01-02 19:50:24 +0000 UTC
		Time: time.Unix(0, 1514922624000000000),
	},
	DataPoint{
		Fields: map[string]IDBFloat64{
			"loss": 5.0,
			"sent": 480.0,
			"lost": 24.0,
			"rtt":  11.3,
		},
		Measurement: "measurement",
		Tags: map[string]string{
			"src_metro": "abc",
			"dst_metro": "def",
		},
		Time: time.Unix(0, 1514922624000000000),
	},
}

// Actual tests
func (s *ScraperSuite) TestNewInfluxDbWriter(c *gocheck.C) {
	writer, err := NewInfluxDbWriter("localhost", "5000", "user", "pass", "dbname")
	c.Assert(err, gocheck.IsNil)
	c.Assert(writer, gocheck.FitsTypeOf, &InfluxDbWriter{})
	writer, err = NewInfluxDbWriter("127.0.0.1", "5000", "", "", "dbname")
	c.Assert(err, gocheck.IsNil)
	c.Assert(writer, gocheck.FitsTypeOf, &InfluxDbWriter{})
}

func (s *ScraperSuite) TestInfluxDbWriter_Write(c *gocheck.C) {
	bp, err := influxdb_client.NewBatchPoints(influxdb_client.BatchPointsConfig{})
	c.Assert(err, gocheck.IsNil)
	err = s.writer.Write(bp)
	c.Assert(err, gocheck.IsNil)
}

func (s *ScraperSuite) TestInfluxDbWriter_Batch(c *gocheck.C) {
	batch, err := s.writer.Batch(examplePoints)
	c.Assert(err, gocheck.IsNil)
	c.Assert(len(batch.Points()), gocheck.Equals, 2)
}

func (s *ScraperSuite) TestInfluxDbWriter_BatchWrite(c *gocheck.C) {
	err := s.writer.BatchWrite(examplePoints)
	c.Assert(err, gocheck.IsNil)
}

func (s *ScraperSuite) TestNewScraper(c *gocheck.C) {
	newS, err := NewScraper([]string{"localhost", "127.0.0.1"}, "5000", "localhost", "5086", "user", "pass", "dbname")
	c.Assert(err, gocheck.IsNil)
	c.Assert(newS, gocheck.FitsTypeOf, &Scraper{})
	newS, err = NewScraper([]string{"1.2.3.4", "127.0.0.1"}, "5000", "127.0.0.1", "5086", "", "", "dbname")
	c.Assert(err, gocheck.IsNil)
	c.Assert(newS, gocheck.FitsTypeOf, &Scraper{})
}

func (s *ScraperSuite) TestScraper_run(c *gocheck.C) {
	for _, collector := range s.scraper.collectors {
		err := s.scraper.run(collector)
		c.Assert(err, gocheck.IsNil)
	}
}
