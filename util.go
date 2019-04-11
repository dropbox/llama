package llama

import (
	"log"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/satori/go.uuid"
	"golang.org/x/sys/unix"
)

const (
	// Listens on any addr to an automatically assigned port number
	DefaultAddrStr        = "0.0.0.0:0"
	DefaultTos            = byte(0)
	DefaultRcvBuff        = 2097600 // 2MiB
	DefaultReadTimeout    = 200 * time.Millisecond
	DefaultCacheTimeout   = 2 * time.Second
	DefaultCacheCleanRate = 5 * time.Second
	ExpireNow             = time.Nanosecond
)

// NewID returns 10 bytes of a new UUID4 as a string.
//
// This should be unique enough for short-lived cases, but as it's only a
// partial UUID4.
func NewID() string {
	fullUUID := uuid.NewV4()
	last10 := fullUUID[len(fullUUID)-10:]
	return string(last10)
}

// IDTo10Bytes converts a string to a 10 byte array.
func IDToBytes(id string) [10]byte {
	var arr [10]byte
	copy(arr[:], id)
	return arr
}

// NowUint64 returns the current time in nanoseconds as a uint64.
func NowUint64() uint64 {
	return uint64(time.Now().UnixNano())
}

// FileCloseHandler will close an open File and handle the resulting error.
func FileCloseHandler(f *os.File) {
	// NOTE: This is required, specifically for sockets/net.Conn because it
	// would appear that calls like setting the ToS value or enabling
	// timestamps cause this to go into a blocking state. Which then disables
	// the functionality of SetReadDeadline, making reads block infinitely.
	err := unix.SetNonblock(int(f.Fd()), true)
	HandleError(err)
	err = f.Close()
	HandleError(err)
}

func HandleError(err error) {
	HandleFatalError(err)
}

func HandleMinorError(err error) {
	if err != nil {
		// Could have this logging to Sentry
		//log.Printf("ERROR TYPE: %#v\n", err)
		// NOTE(dmar): Very annoying downside of this separate method
		//			   approach is that this is the line number which
		//			   appears in logs. Not the actual source.
		log.Println("ERROR: ", err)
	}
}

// HandleError receives an error, then logs and exits if not nil.
// TODO(dmar): Create additional simple handlers for non-fatal issues
func HandleFatalError(err error) {
	if err != nil {
		// Could have this logging to Sentry
		//log.Printf("ERROR TYPE: %#v\n", err)
		log.Fatal("ERROR: ", err)
	}
}

// SetRecvBufferSize sets the size of the receive buffer for the conn to the
// provided size in bytes.
// TODO(dmar): Validate and replace this with a simple call to conn.SetReadBuffer
func SetRecvBufferSize(conn *net.UDPConn, size int) {
	file, err := conn.File()
	defer FileCloseHandler(file)
	HandleError(err)
	err = syscall.SetsockoptInt(int(file.Fd()), syscall.SOL_SOCKET,
		syscall.SO_RCVBUF, size)
	HandleError(err)
}
