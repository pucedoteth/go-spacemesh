package net

import (
	"errors"
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/p2p/config"
	"github.com/spacemeshos/go-spacemesh/p2p/delimited"
	"github.com/spacemeshos/go-spacemesh/p2p/p2pcrypto"
	"time"

	"fmt"
	"io"
	"net"
	"sync"

	"github.com/spacemeshos/go-spacemesh/crypto"
)

var (
	// ErrClosedIncomingChannel is sent when the connection is closed because the underlying formatter incoming channel was closed
	ErrClosedIncomingChannel = errors.New("unexpected closed incoming channel")
	// ErrConnectionClosed is sent when the connection is closed after Close was called
	ErrConnectionClosed = errors.New("connections was intentionally closed")
)

// ConnectionSource specifies the connection originator - local or remote node.
type ConnectionSource int

// ConnectionSource values
const (
	Local ConnectionSource = iota
	Remote
)

type queuedMessage struct {
	b []byte
	res chan error
}


// Connection is an interface stating the API of all secured connections in the system
type Connection interface {
	fmt.Stringer

	ID() string
	RemotePublicKey() p2pcrypto.PublicKey
	SetRemotePublicKey(key p2pcrypto.PublicKey)

	RemoteAddr() net.Addr

	Session() NetworkSession
	SetSession(session NetworkSession)

	Send(m []byte) error
	SendNow(m []byte) error
	Close() error
	Closed() bool
}

// FormattedConnection is an io.Writer and an io.Closer
// A network connection supporting full-duplex messaging
type FormattedConnection struct {
	// metadata for logging / debugging
	logger     log.Log
	id         string // uuid for logging
	created    time.Time
	remotePub  p2pcrypto.PublicKey
	remoteAddr net.Addr
	networker  networker // network context
	session    NetworkSession
	deadline   time.Duration
	timeout    time.Duration
	r          formattedReader
	wmtx       sync.Mutex
	w          formattedWriter
	closed     bool
	deadliner  deadliner
	close      io.Closer

	sendQueue chan queuedMessage

	msgSizeLimit int
}

type networker interface {
	HandlePreSessionIncomingMessage(c Connection, msg []byte) error
	EnqueueMessage(ime IncomingMessageEvent)
	SubscribeClosingConnections(func(c ConnectionWithErr))
	publishClosingConnection(c ConnectionWithErr)
	NetworkID() int8
}

type readWriteCloseAddresser interface {
	io.ReadWriteCloser
	deadliner
	RemoteAddr() net.Addr
}

type deadliner interface {
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
}

type formattedReader interface {
	Next() ([]byte, error)
}

type formattedWriter interface {
	WriteRecord([]byte) (int, error)
}

// Create a new connection wrapping a net.Conn with a provided connection manager
func newConnection(conn readWriteCloseAddresser, netw networker,
	remotePub p2pcrypto.PublicKey, session NetworkSession, msgSizeLimit int, deadline time.Duration, log log.Log) *FormattedConnection {

	// todo parametrize channel size - hard-coded for now
	connection := &FormattedConnection{
		logger:       log,
		id:           crypto.UUIDString(),
		created:      time.Now(),
		remotePub:    remotePub,
		remoteAddr:   conn.RemoteAddr(),
		r:            delimited.NewReader(conn),
		w:            delimited.NewWriter(conn),
		close:        conn,
		deadline:     deadline,
		deadliner:    conn,
		networker:    netw,
		session:      session,
		msgSizeLimit: msgSizeLimit,
		sendQueue: make(chan queuedMessage, 100),
	}

	return connection
}

// ID returns the channel's ID
func (c *FormattedConnection) ID() string {
	return c.id
}

// RemoteAddr returns the channel's remote peer address
func (c *FormattedConnection) RemoteAddr() net.Addr {
	return c.remoteAddr
}

// SetRemotePublicKey sets the remote peer's public key
func (c *FormattedConnection) SetRemotePublicKey(key p2pcrypto.PublicKey) {
	c.remotePub = key
}

// RemotePublicKey returns the remote peer's public key
func (c *FormattedConnection) RemotePublicKey() p2pcrypto.PublicKey {
	return c.remotePub
}

// SetSession sets the network session
func (c *FormattedConnection) SetSession(session NetworkSession) {
	c.session = session
}

// Session returns the network session
func (c *FormattedConnection) Session() NetworkSession {
	return c.session
}

// String returns a string describing the connection
func (c *FormattedConnection) String() string {
	return c.id
}

func (c *FormattedConnection) publish(message []byte) {
	c.networker.EnqueueMessage(IncomingMessageEvent{c, message})
}

// Send binary data to a connection
// data is copied over so caller can get rid of the data
// Concurrency: can be called from any go routine
func (c *FormattedConnection) SendNow(m []byte) error {
	c.wmtx.Lock()
	defer c.wmtx.Unlock()
	if c.closed {
		return fmt.Errorf("connection was closed")
	}

	c.deadliner.SetWriteDeadline(time.Now().Add(c.deadline))
	_, err := c.w.WriteRecord(m)
	if err != nil {
		cerr := c.closeUnlocked()
		if cerr != ErrAlreadyClosed {
			c.networker.publishClosingConnection(ConnectionWithErr{c, err}) // todo: reconsider
		}
		return err
	}
	return nil
}

func (c *FormattedConnection) sendRoutine() {
	for {
		b := <-c.sendQueue
		t := time.Now()
		err := c.SendNow(b.b)
		c.logger.Info("SEND TOOK - %v ", time.Since(t))
		b.res <- err
		if err != nil {
			break
		}
	}
}

func (c * FormattedConnection) Send(m []byte) error {
	c.wmtx.Lock()
	if c.closed {
		c.wmtx.Unlock()
		return fmt.Errorf("connection was closed")
	}
	c.wmtx.Unlock()

	res := make(chan error, 1)
	c.sendQueue <- queuedMessage{m, res }
	return <-res
}

var ErrAlreadyClosed = errors.New("connection is already closed")

func (c *FormattedConnection) closeUnlocked() error {
	if c.closed {
		return ErrAlreadyClosed
	}
	err := c.close.Close()
	c.closed = true
	if err != nil {
		c.logger.Warning("error while closing with connection %v, err: %v", c.RemotePublicKey().String(), err)
		return err
	}
	return nil
}

// Close closes the connection (implements io.Closer). It is go safe.
func (c *FormattedConnection) Close() error {
	c.wmtx.Lock()
	err := c.closeUnlocked()
	c.wmtx.Unlock()
	return err
}

// Closed returns whether the connection is closed
func (c *FormattedConnection) Closed() bool {
	c.wmtx.Lock()
	defer c.wmtx.Unlock()
	return c.closed
}

var ErrTriedToSetupExistingConn = errors.New("tried to setup existing connection")
var ErrIncomingSessionTimeout = errors.New("timeout waiting for handshake message")
var ErrMsgExceededLimit = errors.New("message size exceeded limit")

func (c *FormattedConnection) setupIncoming(timeout time.Duration) error {
	be := make(chan struct {
		b []byte
		e error
	})

	go func() {
		// TODO: some other way to make sure this groutine closes
		c.deadliner.SetReadDeadline(time.Now().Add(60*time.Second))
		msg, err := c.r.Next()
		c.deadliner.SetReadDeadline(time.Time{}) // disable read deadline
		be <- struct {
			b []byte
			e error
		}{b: msg, e: err}
	}()

	t := time.NewTimer(timeout)

	select {
	case msgbe := <-be:
		msg := msgbe.b
		err := msgbe.e

		if err != nil {
			c.Close()
			return err
		}

		if c.msgSizeLimit != config.UnlimitedMsgSize && len(msg) > c.msgSizeLimit {
			c.logger.With().Error("setupIncoming: message is too big",
				log.Int("limit", c.msgSizeLimit), log.Int("actual", len(msg)))
			return ErrMsgExceededLimit
		}

		if c.session != nil {
			c.Close()
			return errors.New("setup connection twice")
		}

		err = c.networker.HandlePreSessionIncomingMessage(c, msg)
		if err != nil {
			c.Close()
			return err
		}
	case <-t.C:
		c.Close()
		return errors.New("timeout while waiting for session message")
	}

	return nil
}

// Push outgoing message to the connections
// Read from the incoming new messages and send down the connection
func (c *FormattedConnection) beginEventProcessing() {
	//TODO: use a buffer pool
	go c.sendRoutine()

	var err error
	var buf []byte
	for {
		buf, err = c.r.Next()
		if err != nil {
			break
		}

		if c.session == nil {
			err = ErrTriedToSetupExistingConn
			break
		}

		newbuf := make([]byte, len(buf))
		copy(newbuf, buf)
		c.publish(newbuf)
	}

	cerr := c.Close()
	if cerr != ErrAlreadyClosed {
		c.networker.publishClosingConnection(ConnectionWithErr{c, err})
	}
}
