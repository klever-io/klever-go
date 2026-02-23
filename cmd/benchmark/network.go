package main

// RunNetworkBenchmark measures TCP loopback characteristics that directly
// affect Kleverchain P2P communication quality:
//
//   - Round-trip latency (P50 / P99): how fast the node can exchange small
//     consensus messages over the loopback interface.  Low latency here
//     indicates a healthy OS networking stack and no CPU-scheduler stall.
//
//   - Streaming throughput (MB/s): sustained data rate through a single TCP
//     connection — relevant for block propagation and snapshot transfers
//     between peers.  Measured receiver-side so kernel buffering does not
//     inflate the result.

import (
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	netPingOps     = 1_000     // round-trips for the latency test
	netStreamMB    = 128       // MB of data for the throughput test
	netStreamBlock = 64 * 1024 // 64 KB send buffer
)

// NetworkResult holds all TCP loopback performance metrics.
type NetworkResult struct {
	LatP50         time.Duration // median round-trip time
	LatP99         time.Duration // 99th-percentile round-trip time
	ThroughputMBps float64       // streaming throughput in MB/s (receiver-side)
}

// RunNetworkBenchmark executes the latency and throughput sub-benchmarks.
func RunNetworkBenchmark() (*NetworkResult, error) {
	result := &NetworkResult{}

	// 1. Latency (ping-pong) ---------------------------------------------------
	clearLine("  Network: TCP loopback latency (%d round-trips)...", netPingOps)
	p50, p99, err := tcpLoopbackLatency()
	if err != nil {
		return nil, fmt.Errorf("tcp latency: %w", err)
	}
	result.LatP50 = p50
	result.LatP99 = p99

	// 2. Throughput (streaming) ------------------------------------------------
	clearLine("  Network: TCP loopback throughput (%d MB)...", netStreamMB)
	mbps, err := tcpLoopbackThroughput()
	if err != nil {
		return nil, fmt.Errorf("tcp throughput: %w", err)
	}
	result.ThroughputMBps = mbps

	fmt.Fprintf(os.Stderr, "  %s\r", strings.Repeat(" ", 60))
	return result, nil
}

// tcpLoopbackLatency measures the round-trip time of 8-byte ping-pong messages
// over a localhost TCP connection and returns the P50 and P99 percentiles.
func tcpLoopbackLatency() (p50, p99 time.Duration, err error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, 0, err
	}
	defer ln.Close()

	// Echo server: reflects every 8-byte message back to the client.
	go func() {
		conn, aerr := ln.Accept()
		if aerr != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 8)
		for {
			if _, rerr := io.ReadFull(conn, buf); rerr != nil {
				return
			}
			if _, werr := conn.Write(buf); werr != nil {
				return
			}
		}
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		return 0, 0, err
	}
	defer conn.Close()

	msg := make([]byte, 8)
	buf := make([]byte, 8)
	rtts := make([]int64, 0, netPingOps)

	for range netPingOps {
		t0 := time.Now()
		if _, werr := conn.Write(msg); werr != nil {
			return 0, 0, werr
		}
		if _, rerr := io.ReadFull(conn, buf); rerr != nil {
			return 0, 0, rerr
		}
		rtts = append(rtts, time.Since(t0).Nanoseconds())
	}

	sort.Slice(rtts, func(i, j int) bool { return rtts[i] < rtts[j] })
	p50 = time.Duration(rtts[len(rtts)*50/100])
	p99 = time.Duration(rtts[len(rtts)*99/100])
	return p50, p99, nil
}

// throughputResult carries the receiver-side measurement from the server goroutine.
type throughputResult struct {
	bytes   int64
	elapsed time.Duration
	err     error
}

// tcpLoopbackThroughput streams netStreamMB of data through a localhost TCP
// connection and returns the throughput in MB/s measured on the receiver side.
func tcpLoopbackThroughput() (float64, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()

	resultCh := make(chan throughputResult, 1)
	go func() {
		conn, aerr := ln.Accept()
		if aerr != nil {
			resultCh <- throughputResult{err: aerr}
			return
		}
		defer conn.Close()
		start := time.Now()
		n, rerr := io.Copy(io.Discard, conn)
		elapsed := time.Since(start)
		if rerr != nil {
			resultCh <- throughputResult{err: rerr}
			return
		}
		resultCh <- throughputResult{bytes: n, elapsed: elapsed}
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		return 0, err
	}

	buf := make([]byte, netStreamBlock)
	for i := range buf {
		buf[i] = byte(i)
	}

	total := int64(netStreamMB * 1024 * 1024)
	var sent int64
	for sent < total {
		remaining := total - sent
		chunk := buf
		if remaining < int64(len(chunk)) {
			chunk = buf[:remaining]
		}
		n, werr := conn.Write(chunk)
		if werr != nil {
			conn.Close()
			return 0, werr
		}
		sent += int64(n)
	}
	conn.Close()

	res := <-resultCh
	if res.err != nil {
		return 0, res.err
	}
	return float64(res.bytes) / (1024 * 1024) / res.elapsed.Seconds(), nil
}
