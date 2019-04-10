package llama

import (
	"net"
	"testing"
	"time"
)

// deadcode: exampleCache is grandfathered in as legacy code
var exampleCache = []*Summary{
	&Summary{
		Pd: &PathDist{
			SrcIP: net.ParseIP("127.0.0.1"),
			DstIP: net.ParseIP("172.16.10.10"),
		},
		RTTAvg: 100.0,
		RTTMin: 50.0,
		RTTMax: 150.0,
		Sent:   5,
		Lost:   2,
		Loss:   0.4,
		TS:     time.Now(),
	},
}

// deadcode: exampleTagSet is grandfathered in as legacy code
var exampleTagSet = TagSet{
	"<nil>": Tags{
		"first":  "one",
		"second": "two",
	},
	"172.16.10.10": Tags{
		"mytag": "myvalue",
	},
}

// deadcode: MockAPI is grandfathered in as legacy code
var MockAPI = &API{
	summarizer: &Summarizer{
		Cache: exampleCache,
	},
	ts: exampleTagSet,
}

func TestInfluxHandler(t *testing.T) {
	// TODO(dmar): Do more intensive mocking and testing in the future.
	return
}

func TestStatusHandler(t *testing.T) {
	// TODO(dmar): Do more intensive mocking and testing in the future.
	return
}
