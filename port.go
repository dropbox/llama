// Functionality for sending and receiving UDP probes on a socket.
package llama

import (
	"errors"
	"log"
	"net"
	"runtime"
	"strings"
	"time"

	gocache "github.com/patrickmn/go-cache"

	pb "github.com/dropbox/go-llama/proto"
)

// Port represents a socket and its associated caching, inputs, and outputs.
type Port struct {
	tosend      chan *net.UDPAddr // A channel for receiving targets
	conn        *net.UDPConn      // The socket on which to send/receive
	cache       *gocache.Cache    // To handle temp storage of probes and timeout
	stop        chan bool         // A signal to stop processing
	cbc         chan *Probe       // Callback channel for sending expired Probes
	readTimeout time.Duration     // How long to wait for reads
	basePD      *PathDist         // A partially filled PathDist based on conn
}

// srcPD creates a PathDist based on the known socket details for the port.
func (p *Port) srcPD() *PathDist {
	if p.basePD != nil {
		// Just return the saved base, so we don't waste time
		return p.basePD
	}
	udpAddr, network, err := LocalUDPAddr(p.conn)
	HandleError(err)
	pd := PathDist{
		SrcIP:   udpAddr.IP,
		SrcPort: udpAddr.Port,
		Proto:   network,
	}
	p.basePD = &pd // Save for future use
	return &pd
}

// pd will provide a completed PathDist based on the associated p.conn and
// the provided net.UDPAddr.
func (p *Port) pd(dst *net.UDPAddr) *PathDist {
	// TODO(dmar): Since many of these are going to be repeats, keep these
	//      around for reuse. That also allows memory pointers to be used
	//      for equality/summarization later.
	// BUG: If generated from ResolveUDPAddr in the form `:<port>` this will
	//      actually be `nil`. However, it's fine if an explicit address
	//      is provided.
	pathDist := &PathDist{
		SrcIP:   p.srcPD().SrcIP,
		SrcPort: p.srcPD().SrcPort,
		Proto:   p.srcPD().Proto,
		DstIP:   dst.IP,
		DstPort: dst.Port,
	}
	return pathDist
}

// ToS provides the currently active ToS byte value for the port's conn.
func (p *Port) Tos() byte {
	val := GetTos(p.conn)
	return val
}

// Send waits to get UDPAddr targets and sends probes to them using the
// associated Port.
//
// After sending the probe, it is added to a cache with a unique ID, which is
// used for retrieving later. The cache will also utilize a timeout to expire
// probes that haven't returned in time.
func (p *Port) Send() {
	go p.send()
}

func (p *Port) send() {
	for {
		select {
		case <-p.stop:
			log.Println("Stopping Port.send for", p.conn.LocalAddr())
			return // Discontinue sending
		case addr := <-p.tosend:
			pd := p.pd(addr)
			tos := p.Tos()
			key := NewID()
			// NOTE: The more time spent before sending, the more stale
			//       this will get. Not critical, but a consideration.
			now := NowUint64()
			probe := Probe{
				Pd:    pd,
				CSent: now,
				Tos:   tos,
			}
			// Add the probe to cache
			// TODO(dmar): Might want to make this async in the future to avoid
			//             making `now` more stale as things are going on.
			p.cache.SetDefault(key, &probe)
			signature := IDToBytes(key)
			var padding [1000]byte
			data := pb.Probe{
				Signature: signature[:],
				Tos:       []byte{tos},
				Sent:      now,
				// TODO(dmar): This should be customizable, and relative to
				//			   to the rest of the probe. This should really
				//             be used to fill to a maximum size.
				//			   Likely based on the return from Marshal.
				Padding: padding[:],
			}
			packedData, err := data.Marshal()
			HandleError(err)
			// Send the probe
			_, err = p.conn.WriteToUDP(packedData, addr)
			HandleError(err)
			// TODO(dmar): Log rate of `packets_sent`
		}
	}
}

// Recv listens on the Port for returning probes and updates them in the cache.
//
// Once probes are received, they are located in the cache, updated, and then
// set for immediate expiration. If a probe is received but has no entry in
// the cache, it most likely exceeded the timeout.
func (p *Port) Recv() {
	go p.recv()
}

func (p *Port) recv() {
	dataBuf := make([]byte, 4096) // Reuse this for the received data
	// This will be implemented for timestamps in the future
	oobBuf := make([]byte, 4096) // Reuse this for the received oob data
	for {
		select {
		case <-p.stop:
			log.Println("Stopping Port.recv for:", p.conn.LocalAddr())
			// Don't process expirations anymore
			// This prevents outstanding probes from reporting as loss
			// NOTE(dmar): Setting this to an empty function instead of nil, as
			//   as the underlying library has a race condition, which can result
			//   in a nil pointer refernce (it calls nil like a function).
			p.cache.OnEvicted(func(s string, i interface{}) {})
			return // Stop receiving
		default:
			// This is a specific point in time, so it needs to be refreshed
			timeout := time.Now().Add(p.readTimeout)
			err := p.conn.SetReadDeadline(timeout)
			HandleError(err)
			// TODO(dmar):
			// This is very similar to `reflector.Receive` except for timeout
			// handling. Should consolidate these at some point in UDP.
			// Ignoring `oobLen` and `flags`for now
			// We don't need `addr since we're matching on the signature
			// NOTE(dmar): For some reason, on stop, every once in a while,
			//   A process will get stuck here. Specifically on the underlying
			//   Recvmsg call in syscall. It seems to ignore the deadline, and
			//   then stick around forever. Unsure of the cause.
			dataLen, _, _, _, err := p.conn.ReadMsgUDP(dataBuf, oobBuf)
			if err != nil {
				// Check if it's a networking error
				netErr, ok := err.(net.Error)
				if ok && netErr.Timeout() {
					// It's a timeout, so we've waited long enough, restart the loop
					continue
				} else if ok && strings.Contains(netErr.Error(),
					"use of closed network connection") {
					// This means the connection is closed, so we can't use it
					// In lieu of better cleanup behavior (for whatever case
					// might cause this) have it cause a restart of the process
					log.Fatal("Attempted to read from closed conn:",
						p.conn.LocalAddr())
					continue
				} else {
					// Some other problem
					log.Fatal("Failure while listening on ", p.conn.LocalAddr(),
						"\n", err.Error())
				}
			}
			data := dataBuf[0:dataLen]
			udpData := &pb.Probe{}
			err = udpData.Unmarshal(data)
			HandleMinorError(err)
			id := string(udpData.Signature[:])
			// TODO(dmar): Should be doing something about this error
			cValue, found := p.cache.Get(id)
			if !found {
				// This means it expired already or doesn't exist
				// so there's nothing to do.
				// TODO(dmar): Log/stat on occurrences of this
				continue
			}
			// TODO(dmar): Make wish to make a `ProbeCache` that does this
			//             automatically under the hood.
			probe, err := IfaceToProbe(cValue)
			HandleMinorError(err)
			// TODO(dmar): Update this to be more clean when moving to protobuf
			probe.CRcvd = NowUint64()
			// Error would be if the key didn't exist, meaning it expired
			// since the Get above. Rare but possible. Acceptable for now.
			// TODO(dmar): Log/stat on occurrences of this
			_ = p.cache.Replace(id, probe, ExpireNow)
			// TODO(dmar): Log rate of `packets_received`
		}
	}
}

// done receives entries in the cache that have expired and passes them to
// the Port's cbc (callback channel)
//
// This basically just exists to the do the type conversion and pass to the
// channel.
func (p *Port) done(key string, value interface{}) {
	probe, err := IfaceToProbe(value)
	HandleMinorError(err)
	p.cbc <- probe
}

// Probe represents a single UDP probe that was sent from, and (hopefully)
// received back, a Port.
type Probe struct {
	Pd    *PathDist
	CSent uint64
	CRcvd uint64
	Tos   byte
}

// PathDist -> Path Distinguisher, uniquely IDs the components that determine
// path selection.
type PathDist struct {
	SrcIP   net.IP
	SrcPort int
	DstIP   net.IP
	DstPort int
	Proto   string // 'udp' generally
}

// Cleanup will close the connection and release the cache.
// This would be triggered as a result of garbage collection, and would likely
// be better suited elsewhere. However, this seems like a fairly simple option
// for now, to avoid needing locks and conflicts between send/recv.
func cleanup(port *Port) {
	log.Println("Started closing port on:", port.conn.LocalAddr())
	err := port.conn.Close()
	HandleMinorError(err)
	// This might not actually be necessary, if we've already stopped
	// using this whole thing. But doesn't hurt either.
	port.cache = nil // Dereference the cache
	log.Println("Finished closing port on:", port.conn.LocalAddr())
}

// New creates and returns a new Port with associated inputs, outputs,
// and caching mechanisms.
func NewPort(conn *net.UDPConn, tosend chan *net.UDPAddr, stop chan bool,
	cbc chan *Probe, cTimeout time.Duration, cCleanRate time.Duration,
	readTimeout time.Duration) *Port {
	// Create the cache
	cache := gocache.New(cTimeout, cCleanRate)
	// Create the port
	port := Port{tosend: tosend, conn: conn, cache: cache,
		stop: stop, cbc: cbc, readTimeout: readTimeout}
	// Used for wrapping the callback channel
	port.cache.OnEvicted(port.done)
	// Ensure that when the port is stopped, we cleanup.
	// This happens on GC, so it may be delayed for a bit.
	runtime.SetFinalizer(&port, cleanup)
	return &port
}

// NewDefault creates a new Port using default settings.
func NewDefault(tosend chan *net.UDPAddr, stop chan bool,
	cbc chan *Probe) *Port {
	// Create a default UDPConn
	udpAddr, err := net.ResolveUDPAddr("udp", DefaultAddrStr)
	HandleError(err)
	udpConn, err := net.ListenUDP("udp", udpAddr)
	HandleError(err)
	// These two are unnecessary, but being explicit
	err = udpConn.SetReadBuffer(DefaultRcvBuff)
	HandleError(err)
	SetTos(udpConn, DefaultTos)
	// TODO(dmar): Update to allow no args, and setting later if desired.
	port := NewPort(
		udpConn,
		tosend,
		stop,
		cbc,
		DefaultCacheTimeout,
		DefaultCacheCleanRate,
		DefaultReadTimeout,
	)
	return port
}

// IfaceToProbe attempts to convert an anonymous object to a Probe, and returns
// and error if the operation failed.
func IfaceToProbe(iface interface{}) (*Probe, error) {
	probe, ok := iface.(*Probe)
	if ok {
		return probe, nil
	} else {
		return probe, errors.New("Object provided is not a Probe")
	}
}
