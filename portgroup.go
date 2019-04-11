// portgroup defines PortGroup, which is used to multiplex UDPAddr structs to
// multiple ports via parallel channels.
package llama

import (
	"log"
	"net"
	"time"
)

type PortGroup struct {
	ports  map[*Port](chan *net.UDPAddr)
	stop   chan bool
	cbc    chan *Probe
	tosend chan *net.UDPAddr
}

// Add will add a Port and channel to the PortGroup.
//
// This must NOT be used after running, as it is currently not threadsafe.
// TODO(dmar): In the future, if doing this is desired, add a mutex and
//      appropriate locking.
func (pg *PortGroup) Add(p *Port, c chan *net.UDPAddr) {
	pg.ports[p] = c
}

// AddNew will create a new Port and add it to the PortGroup via Add.
func (pg *PortGroup) AddNew(portStr string, tos byte, cTimeout time.Duration,
	cCleanRate time.Duration,
	readTimeout time.Duration) (
	*Port, chan *net.UDPAddr) {
	/* Because of typing and how net works, it's just cleaner to pass in a
	   string that identifies the addr/port Oddly enough, passing in a
	   port number and net.IP object would involve more conversions.
	*/
	// Create the address/port we want
	addr, err := net.ResolveUDPAddr("udp", portStr)
	HandleError(err)
	// Grab that socket
	conn, err := net.ListenUDP("udp", addr)
	HandleError(err)
	// Update the ToS value for the socket
	SetTos(conn, tos)
	// Tell the socket to keep timestamps
	// TODO(dmar): Not using this yet, but enabling anyways
	EnableTimestamps(conn)
	// Increase the buffer size, since the default doesn't scale
	// TODO(dmar): This should be configurable higher up, as well want to be
	//             able to tweak this behavior more easily in the config.
	err = conn.SetReadBuffer(DefaultRcvBuff)
	HandleError(err)
	// TODO(dmar): May want to set a global/default buffer size for use here
	input := make(chan *net.UDPAddr, 10)
	// Create the port
	p := NewPort(
		conn,
		input,
		pg.stop,
		pg.cbc,
		cTimeout,
		cCleanRate,
		readTimeout,
	)
	// Add it to the port group
	pg.Add(p, input)
	return p, input
}

// Del removes a Port from the PortGroup.
//
// This must NOT be done after running.
// TODO(dmar): If this is desirable, similar to Add, a mutex and locking
//      will be needed and adds overhead.
func (pg *PortGroup) Del(p *Port) {
	delete(pg.ports, p)
}

// Run will start sending/receiving on all Ports in the PortGroup, and then
// then loop muxing inbound UDPAddrs to all ports until stopped.
//
// TODO(dmar): Add something here to prevent ports from being added after
//      it has started running. Otherwise, a mutex is needed to
//      to sync things, though that may be a fine option as long
//      as there aren't too many goroutines or ports.
// TODO(dmar): Allow an arg for starting multiple goroutines? Otherwise
//      leave that to higher level stuff.
func (pg *PortGroup) Run() {
	// Start all of the ports
	for p := range pg.ports {
		p.Recv()
		p.Send()
	}
	// Start the muxer itself
	go pg.run()
}

func (pg *PortGroup) run() {
	for {
		select {
		case <-pg.stop:
			log.Println("Stopping PortGroup")
			pg.ports = nil
			return // Stop sending and burn it all down
		case addr := <-pg.tosend:
			pg.mux(addr)
		}
	}
}

// mux forwards a UDPAddr to all channels tied to Ports in the PortGroup.
//
// To avoid blocking behavior, if a channel is not ready to receive a UDPAddr
// it will be skipped. This was chosen because blocking on a single port blocks
// all ports, and adding a timeout still slows down everything.
//
// It is NOT currently safe to make additions/removals to the PortGroup after
// it is running. If that is desired in the future, locking will be required
// here, similar to Add and Del.
func (pg *PortGroup) mux(addr *net.UDPAddr) {
	for _, c := range pg.ports {
		// TODO(dmar): Update this with a select and default in the future
		//     if we want to track cases where something breaks here.
		//     Tried it before, but apparently hit some weird issues.
		c <- addr
		continue
	}
}

// Stop will signal all muxing to cease (if started) and stop all Ports.
func (pg *PortGroup) Stop() {
	// Generally, this would be done higher up, but might as well have a call
	// here too for convenience.
	close(pg.stop)
}

// New creates a new PortGroup utilizing a set of input, output, and
// signalling channels.
//
// stop is used to signal stopping of the PortGroup and all ports.
// cbc is used as a callback for completed or timedout probes from all ports.
// tosend is used to receive UDPAddr targets for sending to probes, and is
// muxed across all Ports in the PortGroup.
func NewPortGroup(stop chan bool, cbc chan *Probe,
	tosend chan *net.UDPAddr) *PortGroup {
	pg := PortGroup{
		ports:  make(map[*Port](chan *net.UDPAddr)),
		stop:   stop,
		cbc:    cbc,
		tosend: tosend,
	}
	return &pg
}
