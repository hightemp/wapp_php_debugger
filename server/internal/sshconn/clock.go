package sshconn

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// clockCommand prints the time of the server: seconds and, where date knows %N, nanoseconds.
const clockCommand = "date +%s.%N"

// clockTimeout bounds the measurement: a slow answer says little about the clock anyway.
const clockTimeout = 5 * time.Second

// MeasureClock is how far the server's clock is ahead of ours (negative: behind). The server time
// is compared with the middle of the round trip; with whole seconds only (busybox, BSD date) the
// result is good to about a second. Servers that forbid exec give an error: the offset is unknown.
func MeasureClock(client *ssh.Client) (time.Duration, error) {
	session, err := client.NewSession()
	if err != nil {
		return 0, err
	}
	defer func() { _ = session.Close() }()
	timer := time.AfterFunc(clockTimeout, func() { _ = session.Close() })
	defer timer.Stop()

	sent := time.Now()
	output, err := session.Output(clockCommand)
	received := time.Now()
	if err != nil {
		return 0, fmt.Errorf("%s: %w", clockCommand, err)
	}
	remote, err := parseClock(strings.TrimSpace(string(output)))
	if err != nil {
		return 0, err
	}
	middle := sent.Add(received.Sub(sent) / 2)
	return remote.Sub(middle), nil
}

func parseClock(text string) (time.Time, error) {
	whole, fraction, _ := strings.Cut(text, ".")
	seconds, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return time.Time{}, errors.New("unexpected answer of date: " + text)
	}
	nanos, err := strconv.ParseInt(fraction, 10, 64)
	if err != nil || len(fraction) != 9 {
		// No nanoseconds: the true time is anywhere in this second, take its middle
		return time.Unix(seconds, int64(500*time.Millisecond)), nil
	}
	return time.Unix(seconds, nanos), nil
}
