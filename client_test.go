// Llama client tests
package llama

import (
	"net/http"
	"net/http/httptest"
	"testing"

	gocheck "gopkg.in/check.v1"
)

var test_payload = `
[
    {
        "fields": {
            "loss": 0.0,
            "sent": 480.0,
            "lost": 0.0,
            "rtt": 2.45
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
        "time": "0001-01-01T00:00:00Z",
        "measurement": "stat"
    },
    {
        "fields": {
            "loss": 5.0,
            "sent": 480.0,
            "lost": 24.0,
            "rtt": 23.1
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
        "time": "0001-01-01T00:00:00Z",
        "measurement": "stat"
    }
]
`

// Bootstrap gocheck.
func TestClient(t *testing.T) { gocheck.TestingT(t) }

type ClientSuite struct {
	client Client
	server *httptest.Server
}

var _ = gocheck.Suite(&ClientSuite{})

func (s *ClientSuite) SetUpSuite(c *gocheck.C) {
	s.server = httptest.NewServer(func() http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(test_payload))
		}
	}())
	client := NewClient("localhost", "1234")
	client.getFunc = func(url string) (resp *http.Response, err error) {
		return s.server.Client().Get(s.server.URL)
	}
	s.client = client
}

func (s *ClientSuite) TearDownSuite(c *gocheck.C) {
	s.server.Close()
}

func (s *ClientSuite) SetUpTest(c *gocheck.C) {
}

func (s *ClientSuite) TestGetPoints(c *gocheck.C) {
	points, err := s.client.GetPoints()

	c.Assert(err, gocheck.IsNil)

	// Unpack the 2 datapoints
	p1, p2 := points[0], points[1]

	c.Assert(p1.Measurement, gocheck.Equals, "stat")
	c.Assert(p1.Fields["loss"], gocheck.Equals, IDBFloat64(0.0))

	c.Assert(p2.Measurement, gocheck.Equals, "stat")
	c.Assert(p2.Fields["sent"], gocheck.Equals, IDBFloat64(480.0))

	// Their tags should be identical
	c.Assert(p1.Tags, gocheck.DeepEquals, p2.Tags)
}
