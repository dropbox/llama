package llama

import (
	"fmt"
	"time"
)

// IDBFloat64 is to allow custom JSON marshalling in the API, so it actually
// formats like a float consistently
type IDBFloat64 float64

// TODO(dmar): This should be handled in the scraper by always writing numbers
//             as floats. But for now, ensure that float64 values without
//             decimal precision are still written in decimal format.
//             Otherwise, it turns into an int along the way and makes
//             InfluxDB angry. Another alternative, GRPC and Protobufs instead
//             of a JSON HTTP API.
func (n IDBFloat64) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%f", n)), nil
}

// DataPoint represents a single "point" of data for InfluxDB.
type DataPoint struct {
	Fields      map[string]IDBFloat64 `json:"fields"`
	Tags        Tags             `json:"tags"`
	Time        time.Time             `json:"time"`
	Measurement string                `json:"measurement"`
}

// SetFieldFloat64 sets the value of "field" k to the value v.
func (dp *DataPoint) SetFieldFloat64(k string, v float64) {
	dp.Fields[k] = IDBFloat64(v)
}

// SetFieldINt sets the value of "field" k to the value v.
func (dp *DataPoint) SetFieldInt(k string, v int) {
	dp.Fields[k] = IDBFloat64(v)
}

// FromSummary updates the values of dp to reflect what is available in s.
func (dp *DataPoint) FromSummary(s *Summary) {
	// Populate general fields from the provided summary
	dp.FromPD(s.Pd)
	// NOTE(dmar): Don't set this anymore, and just allow the zero value.
	//      Leaving for posterity incase it's desired in the future.
	dp.SetTime(s.TS)
	dp.SetMeasurement("raw_stats")
	// Set the field values
	// TODO(dmar): Should update `Summary` to have a map of values, and then
	//      just key off of that. Instead of redefining here.
	// TODO(dmar): Actually determine if these other values are useful. We've
	//      never actually used the min/max I don't think we need them.
	//      So getting rid of them for now, and it likely makes sense to
	//      remove them elsewhere in the code as well. If we need more
	//      visibility, stuff like range, stdev, or percentiles, would likely
	//      be better.
	// dp.SetFieldFloat64("rtt_avg", s.RTTAvg)
	// dp.SetFieldFloat64("rtt_min", s.RTTMin)
	// dp.SetFieldFloat64("rtt_max", s.RTTMax)
	dp.SetFieldFloat64("rtt", s.RTTAvg)
	dp.SetFieldFloat64("loss", s.Loss)
	dp.SetFieldInt("lost", s.Lost)
	dp.SetFieldInt("sent", s.Sent)
}

// FromPD updates the values of dp to reflect what is available in pd.
func (dp *DataPoint) FromPD(pd *PathDist) {
	// TODO(dmar): Make these less hard coded, or move out entirely. This feels
	//		like we're enforcing tag structure in code. Ideally we could get
	//		this from the port configurations and convert those fields
	//		from the config. Or something like that.
	dp.Tags["src_ip"] = pd.SrcIP.String()
	dp.Tags["dst_ip"] = pd.DstIP.String()
	// Leave these out for now, as they're going to at many more data points
	// dp.Tags["src_port"] = strconv.Itoa(pd.SrcPort)
	// dp.Tags["dst_port"] = strconv.Itoa(pd.DstPort)
}

// UpdateTags populates the tags of the dp based on the provided Tags map.
func (dp *DataPoint) UpdateTags(t Tags) {
	for k, v := range t {
		dp.Tags[k] = v
	}
}

// SetTime updates the Time of the dp.
func (dp *DataPoint) SetTime(t time.Time) {
	// Set the timestamp
	dp.Time = t
}

// SetMeasurements set the measurement of the dp to the value of s.
func (dp *DataPoint) SetMeasurement(s string) {
	// Set the measurement
	dp.Measurement = s
}

// New provides a new DataPoint populated with values in s and t.
func NewDataPoint(s *Summary, t Tags) *DataPoint {
	dp := &DataPoint{
		Tags:   make(Tags, 0),             // Also to avoid nil
		Fields: make(map[string]IDBFloat64, 0), // Also to avoid nil
	}
	dp.FromSummary(s)
	dp.UpdateTags(t)
	return dp
}

// NewFromSummaries allows bulk operations against New by providing a slice of
// summaries and map of Tags (t).
func NewFromSummaries(summaries []*Summary, t TagSet) []*DataPoint {
	dps := make([]*DataPoint, 0) // Need to do it this way to avoid nil if empty
	for _, s := range summaries {
		dstTags := t[s.Pd.DstIP.String()]
		dps = append(dps, NewDataPoint(s, dstTags))
	}
	return dps
}
