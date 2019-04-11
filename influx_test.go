package llama

import (
	"net"
	"testing"
	"time"
)

func TestSetFieldFloat(t *testing.T) {
	result, _ := IDBFloat64(0.0).MarshalJSON()
	if string(result) != "0.000000" {
		t.Error("Expected 0.000000 but got", string(result))
	}
	result, _ = IDBFloat64(1234.0).MarshalJSON()
	if string(result) != "1234.000000" {
		t.Error("Expected 1234.000000 but got", string(result))
	}
	result, _ = IDBFloat64(1234.5678901234).MarshalJSON()
	if string(result) != "1234.567890" {
		t.Error("Expected 1234.567890 but got", string(result))
	}
}

func TestFromSummary(t *testing.T) {
	dp := NewDataPoint()
	s := &Summary{
		Pd: &PathDist{
			SrcIP: net.ParseIP("127.0.0.1"),
			DstIP: net.IP{},
		},
		RTTAvg: 100.0,
		RTTMin: 50.0,
		RTTMax: 150.0,
		Sent:   5,
		Lost:   2,
		Loss:   0.4,
		TS:     time.Now(),
	}
	dp.FromSummary(s)
	// Check PD
	if dp.Tags["src_ip"] != "127.0.0.1" {
		t.Error("Fields from PathDist not being set")
	}
	// Check time
	if dp.Time.IsZero() {
		t.Error("Time is not being set")
	}
	// Check measurement
	if dp.Measurement != "raw_stats" {
		t.Error("Measurement is not being set")
	}
	// Make sure a couple of fields are being set
	if dp.Fields["rtt"] != 100.0 || dp.Fields["lost"] != 2 {
		t.Error("Fields are not being populated")
	}
}

func TestFromPD(t *testing.T) {
	dp := NewDataPoint()
	pd := &PathDist{
		SrcIP: net.ParseIP("127.0.0.1"),
		DstIP: net.ParseIP("172.16.10.10"),
	}
	dp.FromPD(pd)
	val, found := dp.Tags["src_ip"]
	if !found {
		t.Error("src_ip wasn't populated in DataPoint")
	} else if val != "127.0.0.1" {
		t.Error("For src_ip, expected 127.0.0.1 but found", val)
	}
	val, found = dp.Tags["dst_ip"]
	if !found {
		t.Error("dst_ip wasn't populated in DataPoint")
	} else if val != "172.16.10.10" {
		t.Error("For dst_ip, expected 172.16.10.10 but found", val)
	}
}

func TestUpdateTags(t *testing.T) {
	dp := NewDataPoint()
	tgs := Tags{
		"first":  "one",
		"second": "two",
	}
	dp.UpdateTags(tgs)
	if dp.Tags["first"] != "one" || dp.Tags["second"] != "two" {
		t.Error("Tags not updated correct:", dp.Tags)
	}
}

func TestSetTime(t *testing.T) {
	dp := NewDataPoint()
	ut := time.Unix(1504654423, 0)
	dp.SetTime(ut)
	if dp.Time != ut {
		t.Error("Time was provided for setting, but not applied")
	}
}

func TestNewDataPointFromSummary(t *testing.T) {
	// The bulk of this is tested in TestFromSummary and TestUpdateTags
	tgs := Tags{
		"first":  "one",
		"second": "two",
	}
	s := &Summary{
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
	}
	dp := NewDataPointFromSummary(s, tgs)
	// Other functions will do more detailed checks, just make sure things
	// are getting set.
	if dp.Tags["first"] != "one" || dp.Tags["second"] != "two" {
		t.Error("Tags not updated correct:", dp.Tags)
	}
	if dp.Time.IsZero() {
		t.Error("Time not set")
	}
	if dp.Fields["rtt"] != 100.0 || dp.Fields["lost"] != 2 {
		t.Error("Fields are not being populated:", dp.Fields)
	}
}

func TestNewDataPointsFromSummaries(t *testing.T) {
	s := []*Summary{
		&Summary{Pd: &PathDist{}},
		&Summary{
			Pd: &PathDist{
				DstIP: net.ParseIP("172.16.10.10"),
			},
		},
		&Summary{Pd: &PathDist{}},
	}
	tgs := TagSet{
		"<nil>": Tags{
			"first":  "one",
			"second": "two",
		},
		"172.16.10.10": Tags{
			"mytag": "myvalue",
		},
	}
	dps := NewDataPointsFromSummaries(s, tgs)
	if len(dps) != 3 {
		t.Error("Expected 3 data points, got", len(dps))
	}
	// Again, just make sure basic stuff got populated, and defer to more
	// specific tests for other stuff.
	if dps[0].Tags["first"] != "one" {
		t.Error("Tags not set")
	}
	if dps[1].Tags["mytag"] != "myvalue" {
		t.Error("Tags not set")
	}
	if dps[1].Tags["dst_ip"] != "172.16.10.10" {
		t.Error("PathDist not being processed")
	}
}
