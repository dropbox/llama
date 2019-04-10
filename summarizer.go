package llama

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// Summary represents summaried results and statistics about them.
type Summary struct {
	Pd     *PathDist
	RTTAvg float64
	RTTMin float64
	RTTMax float64
	Sent   int
	Lost   int
	Loss   float64
	TS     time.Time // No longer used, but keeping for posterity
}

// Summarizer stores results and summarizes them at intervals.
type Summarizer struct {
	// NOTE(dmar): For posterity, use value references for mutexes, not pointers
	CMutex   sync.RWMutex
	Cache    []*Summary
	in       chan *Result
	stop     chan bool
	mutex    sync.RWMutex
	results  map[string][]*Result
	interval time.Duration // Keep this, or just pass to `Run`?
	ticker   *time.Ticker
}

// Run causes the summarizer to infinitely wait for new results, store them,
// and then summarize at an interval.
//
// When results are summarized, they are removed and won't be summarized again.
func (s *Summarizer) Run() {
	go s.waitToSummarize()
	// TODO(dmar): Need to make the number of `store` goroutines customizable.
	//      These need to be able to keep up with the ResultHandler(s) and
	//      however many probes are coming from all the ports/testrunners.
	//      This was proving to be a bottleneck before.
	go s.store()
	go s.store()
}

// waitToSummarize will wait until the next full even interval has passed
// and then summarize the stored results into a cache.
//
// The sumarization will happen a even intervals. It also waits until at least
// a full interval has passed before summarize. So the first set of summaries
// will likely emcompass more results than normal.
func (s *Summarizer) waitToSummarize() {
	// Delay initially so it starts on an even interval
	i := int64(s.interval)
	// Sleep until the first interval
	time.Sleep(time.Duration(i - (time.Now().UnixNano() % i)))
	// This just starts the ticker, but doesn't actually start a summary cycle
	// immediately. This allows at least a full cycle of results to populate
	// before the first summarization. So the first summarization will likely
	// cover more results than subsequent ones.
	log.Printf("Starting ticker for Summarizer at %v intervals\n", s.interval)
	s.ticker = time.NewTicker(s.interval)
	// Now loop infinitely waiting for ticks
	for {
		select {
		case <-s.stop:
			return
		case <-s.ticker.C:
			log.Println("Summarizing results")
			s.summarize()
			log.Println("Summarization complete")
		}
	}
}

// summarize pull out the current results, resetting the Summarizer's results,
// and performing summarizations of all the extracted results.
func (s *Summarizer) summarize() {
	// TODO(dmar): May want to time this in the future, and keep track of it
	s.mutex.Lock()
	// Extract the results and reset the map
	results := s.results
	log.Println("Found", len(results), "results to summarize")
	s.results = make(map[string][]*Result)
	s.mutex.Unlock()
	// Create a new cache for this batch of results
	var newCache []*Summary
	// Perform summaries and save to new cache
	for _, results := range results {
		summary := s.summarizeSet(results)
		newCache = append(newCache, summary)
	}
	// Lock and swap the existing cache out for the new summaries
	s.CMutex.Lock()
	s.Cache = newCache
	s.CMutex.Unlock()
}

// summarizeSet will return a Summary for a single set of Results, all of
// which are *assumed* to have the same PathDist[inguisher].
//
// Any desired summarization functions should be called from here, and operate
// on the summary and results.
func (s *Summarizer) summarizeSet(results []*Result) *Summary {
	// TODO(dmar): Fix this bit in the future based on improved handling of
	//             PathDist for keying. For now, since all of them should have
	//             the same Pd, just grab the first one.
	// This would fail if the results were empty, but then there shouldn't
	// be any.
	pd := results[0].Pd
	// NOTE(dmar): If we need timestamps again, this is the place to add them.
	// summary := &Summary{Pd: pd, TS: time.Now()}
	summary := &Summary{Pd: pd}
	// Perform the calculations
	CalcCounts(results, summary)
	CalcLoss(summary)
	CalcRTT(results, summary)
	return summary
}

// store infinitely waits for inbould Results and adds them to the Summarizer's
// results for later summarization.
func (s *Summarizer) store() {
	for {
		// Get the results
		select {
		case <-s.stop:
			return // We're done here
		case result := <-s.in:
			s.addResult(result)
		}
	}
}

// addResult adds a single result to the Summarizer's set of results.
func (s *Summarizer) addResult(result *Result) {
	// TODO(dmar): Since the Pd is a pointer, and isn't shared between Probes
	//      or results, it's different for each, even if they should be the
	//      same. In the future, it may be better to handle that at the Port
	//      level, so that pointers are saved and reused.
	//
	//      Alternatively, don't store as Net.IP, and use a string for the IPs
	//      instead. That way the value of the Pd can be used as a key. But
	//      that seems kinda inefficient.
	//
	//      For now, parse it as a string, as that should be fairly equivalent.
	//      And then populate the Pd pointer based on the value in one of the
	//      Result structs.
	// For now, just keying this on the src/dst IPs to avoid extra points.
	// TODO(dmar): In the future, based on how the above todo turns out,
	//      perhaps customize what fields are used/ignored.
	key := fmt.Sprintf("src_%v->dst_%v", result.Pd.SrcIP, result.Pd.DstIP)
	s.mutex.Lock()
	s.results[key] = append(s.results[key], result)
	// This is simple and frequent, so avoiding the defer overhead
	s.mutex.Unlock()
}

// Stop will stop the summarizer from receiving results or summarizing them.
func (s *Summarizer) Stop() {
	select {
	case <-s.stop: // Provide a safe out if already stopped
	default:
		log.Println("Stopping Summarizer")
		s.ticker.Stop()
		close(s.stop)
	}
}

// New returns a new Summarizer, based on the provided parameters.
func NewSummarizer(in chan *Result, interval time.Duration) *Summarizer {
	stop := make(chan bool)
	results := make(map[string][]*Result)
	summarizer := &Summarizer{
		in:       in,
		stop:     stop,
		results:  results,
		interval: interval,
	}
	return summarizer
}

// CalcRT will calculate the RTT values for the provided summary, based on the
// provided results.
func CalcRTT(results []*Result, summary *Summary) {
	// Up to here, values for RTT are in nanoseconds. However, converting to
	// milliseconds here for backward compatibility and human readability.
	//
	// TODO(dmar): Stop converting to ms and just leave as ns. This will need
	//             some type conversions in the structs and graph updates, but
	//             keeps things in the form they are originally determined in.
	//             Plus it just gets us more precision.
	//
	// TODO(dmar): Similar to before, these are zero if everything was lost.
	//             See CalcLoss for the issue regarding NaN. So need to
	//             determine how best to handle this.
	// If there are no results, abort
	if len(results) == 0 {
		return
	}
	var values []float64
	// Extract the RTT values and convert
	for _, r := range results {
		// If lost, don't include it
		if r.Lost {
			continue
		}
		// Also converting to milliseconds here
		values = append(values, NsToMs(float64(r.RTT)))
	}

	// If no tests actually completed, just end here
	if len(values) == 0 {
		// TODO(dmar): This will leave them all as the zero values, which is
		//      0.0 for now. See other comments about this behavior.
		return
	}
	// Get the average
	total := 0.0
	for _, v := range values {
		total += v
	}
	avg := total / float64(len(values))
	summary.RTTAvg = avg
	// Get the min
	min := math.MaxFloat64
	for _, v := range values {
		if v < min {
			min = v
		}
	}
	summary.RTTMin = min
	// Get the max
	max := 0.0
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	summary.RTTMax = max
}

// CalcCounts will calculate the Sent and Lost counts on the provided summary,
// based on the provided results.
func CalcCounts(results []*Result, summary *Summary) {
	// These opt are safe for an empty slice, so avoiding extra logic
	summary.Sent = len(results)
	lost := 0
	for _, r := range results {
		if r.Lost {
			lost++
		}
	}
	summary.Lost = lost
}

// CalcLoss will calculate the Loss percentage (out of 1) based on the Sent
// and Lost vaules of the provided summary.
func CalcLoss(summary *Summary) {
	// CalcCounts should be called before this, otherwise we're just using the
	// zero values.
	if summary.Sent == 0 {
		// So math.NaN() is not supported by json.Marshall. So this is messy
		// regardless of what option is chosen. Either customizer the
		// marshaller, treat as zero, or make it a pointer so we get nil.
		// Doing zero for now, as there's technically no loss.
		summary.Loss = 0
	}
	// TODO(dmar): Following the existing pattern by converting this to
	//      percent out of 100 instead of 1. It's just extra math, but not
	//      impactful enough to really justify dealing with.
	summary.Loss = (float64(summary.Lost) / float64(summary.Sent)) * 100.0
}

// NsToMs takes ns (nanoseconds) and converts it to milliseconds.
func NsToMs(ns float64) float64 {
	return ns / 1000000.0
}
