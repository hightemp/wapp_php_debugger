package sshconn

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Connection states reported by Pool.Status and PoolOptions.OnStatus.
const (
	StateUnknown    = "unknown" // no attempt yet
	StateConnecting = "connecting"
	StateOnline     = "online"
	StateOffline    = "offline"
)

// Status is the connection state of a server.
type Status struct {
	State string
	Error string // why the server is offline
	Since time.Time
	// ClockOffset is how far the server's clock is ahead of ours, measured on connect; nil when unknown
	ClockOffset *time.Duration
}

// Conn is the shared connection to a server: SSH for exec, SFTP for files.
type Conn struct {
	SSH  *ssh.Client
	SFTP *sftp.Client
}

// PoolOptions tune the pool; zero values take the defaults.
type PoolOptions struct {
	Keepalive  time.Duration                  // how often a live connection is checked; 30 s
	FirstRetry time.Duration                  // wait after the first failure, doubled each time; 1 s
	MaxRetry   time.Duration                  // the longest wait between attempts; 1 min
	OnStatus   func(serverID int64, s Status) // called on every change of state, outside the pool's lock
}

// Pool keeps one connection per server for all its projects, checks it with keepalives,
// reconnects on demand after a drop and backs off after failed attempts.
type Pool struct {
	dialer  *Dialer
	options PoolOptions
	mu      sync.Mutex
	entries map[int64]*entry
	closed  bool
}

type entry struct {
	conn       *Conn
	dialing    chan struct{} // closed when the attempt in progress ends
	status     Status
	failures   int
	retryAt    time.Time
	lastErr    error
	generation int // a new connection invalidates the watcher of the old one
	stop       chan struct{}
	clock      *time.Duration // offset of the server's clock, measured on connect
}

// NewPool creates a pool dialing with d.
func NewPool(d *Dialer, options PoolOptions) *Pool {
	if options.Keepalive == 0 {
		options.Keepalive = 30 * time.Second
	}
	if options.FirstRetry == 0 {
		options.FirstRetry = time.Second
	}
	if options.MaxRetry == 0 {
		options.MaxRetry = time.Minute
	}
	return &Pool{dialer: d, options: options, entries: map[int64]*entry{}}
}

// Get returns the connection to the server, connecting when there is none. During the back-off
// after a failure it returns the last error at once.
func (p *Pool) Get(ctx context.Context, serverID int64, target Target) (*Conn, error) {
	p.mu.Lock()
	for {
		if p.closed {
			p.mu.Unlock()
			return nil, errors.New("ssh pool is closed")
		}
		e := p.entry(serverID)
		if e.conn != nil {
			conn := e.conn
			p.mu.Unlock()
			return conn, nil
		}
		if e.dialing != nil {
			wait := e.dialing
			p.mu.Unlock()
			select {
			case <-wait:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			p.mu.Lock()
			continue
		}
		if wait := time.Until(e.retryAt); wait > 0 && e.lastErr != nil {
			err := fmt.Errorf("%w; retry in %s", e.lastErr, wait.Round(time.Second))
			p.mu.Unlock()
			return nil, err
		}
		break
	}
	e := p.entries[serverID]
	attempt := make(chan struct{})
	e.dialing = attempt
	notify := p.setStatus(serverID, e, StateConnecting, nil)
	p.mu.Unlock()
	notify()

	// One caller giving up must not fail the attempt the others wait for; the dialer has a timeout
	conn, err := p.connect(context.WithoutCancel(ctx), target)
	var clock *time.Duration
	if err == nil {
		if offset, clockErr := MeasureClock(conn.SSH); clockErr == nil {
			clock = &offset
		}
	}

	p.mu.Lock()
	e.dialing = nil
	close(attempt)
	if err == nil && p.closed {
		closeConn(conn)
		err = errors.New("ssh pool is closed")
	}
	if err != nil {
		e.failures++
		e.lastErr = err
		e.retryAt = time.Now().Add(p.backoff(e.failures))
		notify = p.setStatus(serverID, e, StateOffline, err)
		p.mu.Unlock()
		notify()
		return nil, err
	}
	e.conn = conn
	e.clock = clock
	e.failures = 0
	e.lastErr = nil
	e.generation++
	e.stop = make(chan struct{})
	go p.watch(serverID, e.generation, conn, e.stop)
	notify = p.setStatus(serverID, e, StateOnline, nil)
	p.mu.Unlock()
	notify()
	return conn, nil
}

// Status is the current state of the server's connection.
func (p *Pool) Status(serverID int64) Status {
	p.mu.Lock()
	defer p.mu.Unlock()
	if e, ok := p.entries[serverID]; ok {
		return e.status
	}
	return Status{State: StateUnknown}
}

// Reset forgets the back-off, so the next Get dials at once (after a host key was confirmed, say).
func (p *Pool) Reset(serverID int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if e, ok := p.entries[serverID]; ok {
		e.failures = 0
		e.retryAt = time.Time{}
	}
}

// Drop closes the server's connection (the server was removed or its settings changed).
func (p *Pool) Drop(serverID int64) {
	p.mu.Lock()
	e, ok := p.entries[serverID]
	if !ok || e.conn == nil {
		p.mu.Unlock()
		return
	}
	p.release(e)
	notify := p.setStatus(serverID, e, StateOffline, nil)
	p.mu.Unlock()
	notify()
}

// Close closes every connection; Get fails afterwards.
func (p *Pool) Close() {
	p.mu.Lock()
	p.closed = true
	for _, e := range p.entries {
		if e.conn != nil {
			p.release(e)
		}
	}
	p.mu.Unlock()
}

func (p *Pool) entry(serverID int64) *entry {
	e, ok := p.entries[serverID]
	if !ok {
		e = &entry{status: Status{State: StateUnknown}}
		p.entries[serverID] = e
	}
	return e
}

// release closes the connection of e; the caller holds the lock.
func (p *Pool) release(e *entry) {
	close(e.stop)
	closeConn(e.conn)
	e.conn = nil
	e.generation++
}

func (p *Pool) backoff(failures int) time.Duration {
	delay := p.options.FirstRetry
	for i := 1; i < failures && delay < p.options.MaxRetry; i++ {
		delay *= 2
	}
	return min(delay, p.options.MaxRetry)
}

// setStatus records the state; the returned function reports it and must be called without the lock.
func (p *Pool) setStatus(serverID int64, e *entry, state string, err error) func() {
	status := Status{State: state, Since: time.Now()}
	if err != nil {
		status.Error = err.Error()
	}
	if state == StateOnline {
		status.ClockOffset = e.clock
	}
	if e.status.State == state && e.status.Error == status.Error {
		return func() {}
	}
	e.status = status
	if p.options.OnStatus == nil {
		return func() {}
	}
	return func() { p.options.OnStatus(serverID, status) }
}

func (p *Pool) connect(ctx context.Context, target Target) (*Conn, error) {
	client, err := p.dialer.Dial(ctx, target)
	if err != nil {
		return nil, err
	}
	files, err := sftp.NewClient(client)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("sftp on %s: %w", target.Address(), err)
	}
	return &Conn{SSH: client, SFTP: files}, nil
}

// watch notices a closed connection or one that stopped answering keepalives.
func (p *Pool) watch(serverID int64, generation int, conn *Conn, stop chan struct{}) {
	closed := make(chan struct{})
	go func() {
		_ = conn.SSH.Wait()
		close(closed)
	}()
	ticker := time.NewTicker(p.options.Keepalive)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-closed:
			p.lost(serverID, generation, errors.New("connection closed"))
			return
		case <-ticker.C:
			if err := keepalive(conn.SSH, max(p.options.Keepalive, 10*time.Second)); err != nil {
				p.lost(serverID, generation, err)
				return
			}
		}
	}
}

// keepalive sends the request OpenSSH uses; any answer, even a refusal, means the server is alive.
func keepalive(client *ssh.Client, timeout time.Duration) error {
	answer := make(chan error, 1)
	go func() {
		_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
		answer <- err
	}()
	select {
	case err := <-answer:
		return err
	case <-time.After(timeout):
		return errors.New("no answer to keepalive")
	}
}

func (p *Pool) lost(serverID int64, generation int, err error) {
	p.mu.Lock()
	e, ok := p.entries[serverID]
	if !ok || e.generation != generation || e.conn == nil {
		p.mu.Unlock()
		return
	}
	p.release(e)
	e.lastErr = err
	e.retryAt = time.Time{} // a working connection dropped: the next Get reconnects at once
	notify := p.setStatus(serverID, e, StateOffline, err)
	p.mu.Unlock()
	notify()
}

func closeConn(conn *Conn) {
	_ = conn.SFTP.Close()
	_ = conn.SSH.Close()
}
