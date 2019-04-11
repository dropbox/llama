package llama

import (
	"fmt"
	"math"
	"testing"
	"time"
)

func TestWaitToSummarize(t *testing.T) {
	// Could do a bit more testing with some mocking
	s := Summarizer{
		stop:     make(chan bool),
		interval: time.Nanosecond,
	}
	close(s.stop) // Close the channel immediately so we exit
	s.waitToSummarize()
	// Make sure the ticker got created
	if s.ticker == nil {
		t.Error("Ticker wasn't created")
	}
	s.ticker.Stop() // Stop the ticker
}

func TestSummarize(t *testing.T) {
	// With mocking, we could test this more completed, but for now, avoid
	// also covering the other summarize steps
	// Setup
	s := Summarizer{}
	results := make(map[string][]*Result)
	s.results = results
	// Make sure the results got replaced after summarize
	s.summarize()
	if &s.results == &results {
		t.Error("Results on summarizer not reset between runs")
	}
}

func TestSummarizeSet(t *testing.T) {
	s := Summarizer{}
	// Create some fake results
	key := "test"
	s.results = make(map[string][]*Result)
	s.results[key] = append(s.results[key], &Result{RTT: 1000000})
	s.results[key] = append(s.results[key], &Result{Lost: true})
	s.results[key] = append(s.results[key], &Result{RTT: 3000000})
	// Summarize
	summary := s.summarizeSet(s.results[key])
	// Validate results
	if summary.RTTAvg != 2.0 {
		t.Error("RTTAvg bad. Got", summary.RTTAvg, "expected", 2.0)
	}
	if summary.RTTMin != 1.0 {
		t.Error("RTTMin bad. Got", summary.RTTMin, "expected", 1.0)
	}
	if summary.RTTMax != 3.0 {
		t.Error("RTTMax bad. Got", summary.RTTMax, "expected", 3.0)
	}
	if summary.Sent != 3 {
		t.Error("Sent bad. Got", summary.Sent, "expected", 3)
	}
	if summary.Lost != 1 {
		t.Error("Lost bad. Got", summary.Lost, "expected", 1)
	}
	expectedLoss := (float64(1) / float64(3)) * float64(100)
	if summary.Loss != expectedLoss {
		t.Error("Loss bad. Got", summary.Loss, "expected", expectedLoss)
	}
	// NOTE(dmar): Keeping, because this code is still there but commented out.
	//      However, we aren't setting this anymore, and explicitly leaving it
	//      as zero.
	// if summary.TS.IsZero() {
	// 	t.Error("Summary TS was not initialized")
	// }
}

func TestStore(t *testing.T) {
	// This is basically just a loop that reads from a channel
}

func TestAddResult(t *testing.T) {
	// Mock
	s := Summarizer{}
	s.results = make(map[string][]*Result)
	// Add a result
	result := &Result{
		Pd: &PathDist{},
	}
	s.addResult(result)
	// Make sure the result exists
	key := fmt.Sprintf("src_%v->dst_%v", result.Pd.SrcIP, result.Pd.DstIP)
	if len(s.results[key]) != 1 {
		t.Error("Results should contain one entry, but has", len(s.results[key]))
	}
	if s.results[key][0] != result {
		t.Error("The entry in results doesn't match what was provided")
	}
}

func TestSummarizerStop(t *testing.T) {
	s := Summarizer{
		stop:   make(chan bool),
		ticker: time.NewTicker(time.Nanosecond),
	}
	// Stop things
	s.Stop()
	// The ticker should be stopped as well, but there's no good way to verify
	// Make sure the stop channel is closed
	select {
	case <-s.stop:
	default:
		t.Error("Stop didn't actually stop")
	}
}

func TestNewSummarizer(t *testing.T) {
	// Just make sure we can create one
	summarizer := NewSummarizer(
		make(chan *Result),
		time.Second,
	)
	if summarizer == nil {
		t.Error("Was unable to create a Summarizer")
	}
}

func TestCalcRTT(t *testing.T) {
	// These are generally handled under TestSummarizeSet, so add more specific
	// tests and corner cases here.
	summary := &Summary{}
	var results []*Result
	// Validate the empty set and zero values
	CalcRTT(results, summary)
	if summary.RTTAvg != 0.0 || summary.RTTMin != 0.0 || summary.RTTMax != 0.0 {
		t.Error("Zero values for RTT don't appear to be 0.0. Got",
			summary.RTTAvg, summary.RTTMin, summary.RTTMax)
	}

	// Validate that RTT zero values are used if all probes are lost
	summary = &Summary{}
	results = results[:0]
	// This should never happen, but adding to ensure the value isn't used
	results = append(results, &Result{RTT: 1000, Lost: true})
	results = append(results, &Result{Lost: true})
	results = append(results, &Result{Lost: true})
	CalcRTT(results, summary)
	if summary.RTTAvg != 0.0 || summary.RTTMin != 0.0 || summary.RTTMax != 0.0 {
		t.Error("Expected 0.0 for summary values after total loss. Got",
			summary.RTTAvg, summary.RTTMin, summary.RTTMax)
	}

	// Make sure they're all the same if only a single value
	summary = &Summary{}
	results = results[:0]
	results = append(results, &Result{RTT: 2000000})
	CalcRTT(results, summary)
	if summary.RTTAvg != 2.0 || summary.RTTMin != 2.0 || summary.RTTMax != 2.0 {
		t.Error("Calculated values for RTT don't appear to be 2.0. Got",
			summary.RTTAvg, summary.RTTMin, summary.RTTMax)
	}
}

func TestCalcCounts(t *testing.T) {
	// These are generally handled under TestSummarizeSet, so add more specific
	// tests and corner cases here.
	summary := &Summary{}
	// Test with empty slice
	var results []*Result
	CalcCounts(results, summary)
	if summary.Sent != 0 {
		t.Error("Expected sent to be 0, got ", summary.Sent)
	}
	if summary.Lost != 0 {
		t.Error("Expected lost to be 0, got ", summary.Lost)
	}
	// Test mix
	summary = &Summary{}
	results = results[:0]
	results = append(results, &Result{})
	results = append(results, &Result{Lost: true})
	results = append(results, &Result{Lost: true})
	CalcCounts(results, summary)
	if summary.Sent != 3 {
		t.Error("Expected sent to be 3, got ", summary.Sent)
	}
	if summary.Lost != 2 {
		t.Error("Expected lost to be 2, got ", summary.Lost)
	}
	// Test all lost
	summary = &Summary{}
	results = results[:0]
	results = append(results, &Result{Lost: true})
	results = append(results, &Result{Lost: true})
	results = append(results, &Result{Lost: true})
	results = append(results, &Result{Lost: true})
	CalcCounts(results, summary)
	if summary.Sent != 4 {
		t.Error("Expected sent to be 4, got ", summary.Sent)
	}
	if summary.Lost != 4 {
		t.Error("Expected lost to be 4, got ", summary.Lost)
	}
}

func TestCalcLoss(t *testing.T) {
	// These are generally handled under TestSummarizeSet, so add more specific
	// tests and corner cases here.
	s := &Summary{Sent: 5, Lost: 1}
	CalcLoss(s)
	expected := (1.0 / 5.0) * 100.0
	if s.Loss != expected {
		t.Error("Loss calculation incorrect. Expected", expected, "but got", s.Loss)
	}
	// Empty set
	s = &Summary{}
	CalcLoss(s)
	if math.IsNaN(s.Loss) != true {
		t.Error("Loss should be NaN if none sent. Got ", s.Loss)
	}
	// No loss
	s = &Summary{Sent: 5}
	CalcLoss(s)
	expected = (0.0 / 5.0) * 100
	if s.Loss != expected {
		t.Error("Loss calculation incorrect. Expected", expected, "but got", s.Loss)
	}
}
