package sshconn

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/sshconn/sshtest"
)

func near(got, want, tolerance time.Duration) bool {
	return got >= want-tolerance && got <= want+tolerance
}

func TestClockOffsetOfAServerAhead(t *testing.T) {
	ahead := time.Now().Add(90 * time.Second)
	server := sshtest.Start(t, sshtest.WithExec(clockCommand, fmt.Sprintf("%d.%09d\n", ahead.Unix(), ahead.Nanosecond())))
	client, err := dialer(t).Dial(context.Background(), targetOf(server, knownHostsFile(t, server)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	offset, err := MeasureClock(client)
	if err != nil || !near(offset, 90*time.Second, 500*time.Millisecond) {
		t.Fatalf("offset = %v, %v; want about 90s", offset, err)
	}
}

func TestClockOffsetInSecondsWhereDateHasNoNanoseconds(t *testing.T) {
	behind := time.Now().Add(-30 * time.Second)
	// busybox and BSD date print %N as it is
	server := sshtest.Start(t, sshtest.WithExec(clockCommand, fmt.Sprintf("%d.N\n", behind.Unix())))
	client, err := dialer(t).Dial(context.Background(), targetOf(server, knownHostsFile(t, server)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	offset, err := MeasureClock(client)
	if err != nil || !near(offset, -30*time.Second, 1500*time.Millisecond) {
		t.Fatalf("offset = %v, %v; want about -30s", offset, err)
	}
}

func TestClockOffsetUnknownWithoutExec(t *testing.T) {
	server := sshtest.Start(t) // no date command
	client, err := dialer(t).Dial(context.Background(), targetOf(server, knownHostsFile(t, server)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	if _, err := MeasureClock(client); err == nil {
		t.Fatal("no date on the server, yet an offset")
	}
}

func TestPoolReportsTheClockOffset(t *testing.T) {
	ahead := time.Now().Add(5 * time.Second)
	server := sshtest.Start(t, sshtest.WithExec(clockCommand, fmt.Sprintf("%d.%09d\n", ahead.Unix(), ahead.Nanosecond())))
	pool, _ := newPool(t, time.Minute)
	if _, err := pool.Get(context.Background(), 1, targetOf(server, knownHostsFile(t, server))); err != nil {
		t.Fatal(err)
	}
	status := pool.Status(1)
	if status.ClockOffset == nil || !near(*status.ClockOffset, 5*time.Second, 500*time.Millisecond) {
		t.Fatalf("status = %+v", status)
	}

	quiet := sshtest.Start(t)
	if _, err := pool.Get(context.Background(), 2, targetOf(quiet, knownHostsFile(t, quiet))); err != nil {
		t.Fatal(err)
	}
	if pool.Status(2).ClockOffset != nil {
		t.Fatal("an unknown offset is reported as known")
	}
}
