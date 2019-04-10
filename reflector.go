package llama

import (
    pb "github.com/dropbox/go-llama/proto"
    "log"
    "net"
	"time"
	"golang.org/x/time/rate"
)

// Reflect will listen on the provided UDPConn and will send back any UdpData
// compliant packets that it receives, in compliance with the RateLimiter.
func Reflect(conn *net.UDPConn, rl *rate.Limiter) {
    /*
       NOTE: This function assumes is has exclusive control and may improperly
             set the ToS bits if used in multiple routines. If that behavior is
             desired then the UDPConn will need to be placed in a struct with
             the indicator of ToS value. That would be much more efficient than
             checking the current value for each run.
    */
    dataBuf := make([]byte, 4096)
    oobBuf := make([]byte, 4096)
    tos := byte(0)

    log.Println("Beginning reflection on:", conn.LocalAddr())
    for {
		// Use reserve so we can track when trottling happens
		reservation := rl.Reserve()
		delay := reservation.Delay()
		if delay > 0 {
			// We hit the rate limit, so log it
			// TODO(dmar): Log rate of `throttled`
			time.Sleep(delay)
		}

        // Receive data from the connection
        // Not currently using `oob`
        data, _, addr := Receive(dataBuf, oobBuf, conn)

        // For this section, it might make sense to put in `Process` anyways.
        // But for now, all we need is to make sure it's llama data
        // and get the ToS value.
        pbProbe := &pb.Probe{}
        err := pbProbe.Unmarshal(data)
        if err != nil {
            // Else, don't reflect bad data
            log.Println("Error hit when unmarshalling probe")
            //TODO(dmar): Log rate of `packets_bad_data`
            HandleMinorError(err)
            continue
        }

        // Update the ToS (if needed)
        if tos != pbProbe.Tos[0] {
            // Update the connection's ToS value
            SetTos(conn, pbProbe.Tos[0])
            tos = pbProbe.Tos[0]
        }

        // Send the data back to sender
        Send(data, conn, addr)
        //TODO(dmar): Log rate of `packets_processed`
    }
}

// Receive accepts UDP packets on the provided conn and returns the data and
// and control message slices, as well as the UDPAddr it was received from.
func Receive(data []byte, oob []byte, conn *net.UDPConn) (
    []byte, []byte, *net.UDPAddr) {
    // Receive the data from the connection
    dataLen, oobLen, _, addr, err := conn.ReadMsgUDP(data, oob)
    HandleError(err)
    return data[0:dataLen], oob[0:oobLen], addr
}

// Send will send the provided data using the conn to the addr, via UDP.
func Send(data []byte, conn *net.UDPConn, addr *net.UDPAddr) {
    _, err := conn.WriteToUDP(data, addr)
    HandleError(err)
}
