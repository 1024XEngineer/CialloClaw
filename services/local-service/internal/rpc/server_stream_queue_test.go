// RPC stream queue tests verify pending-request backpressure and disconnect handling.
package rpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

func TestHandleStreamConnAppliesBackpressureWhenPendingQueueFills(t *testing.T) {
	server := newTestServer()
	startedSignals := make(chan struct{}, maxPendingStreamRequests+1)
	releaseBlocking := make(chan struct{})
	releasedBlocking := false
	defer func() {
		if !releasedBlocking {
			close(releaseBlocking)
		}
	}()

	var startedMu sync.Mutex
	startedCount := 0
	server.handlers["test.blocking"] = func(_ map[string]any) (any, *rpcError) {
		startedMu.Lock()
		startedCount++
		startedMu.Unlock()

		startedSignals <- struct{}{}
		<-releaseBlocking
		return map[string]any{"status": "released"}, nil
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen loopback: %v", err)
	}
	defer listener.Close()

	acceptDone := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			acceptDone <- err
			return
		}
		server.handleStreamConn(conn)
		acceptDone <- nil
	}()

	right, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial loopback: %v", err)
	}
	defer func() {
		_ = right.Close()
		select {
		case err := <-acceptDone:
			if err != nil && !errors.Is(err, net.ErrClosed) {
				t.Fatalf("accept loopback: %v", err)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("expected loopback stream to shut down")
		}
	}()

	encoder := json.NewEncoder(right)
	for index := 0; index < maxPendingStreamRequests; index++ {
		request := requestEnvelope{
			JSONRPC: "2.0",
			ID:      json.RawMessage(fmt.Sprintf(`"req-blocking-%d"`, index)),
			Method:  "test.blocking",
			Params:  mustMarshal(t, map[string]any{}),
		}
		if err := encoder.Encode(request); err != nil {
			t.Fatalf("encode blocking request %d: %v", index, err)
		}
	}

	for index := 0; index < maxPendingStreamRequests; index++ {
		select {
		case <-startedSignals:
		case <-time.After(2 * time.Second):
			t.Fatalf("expected request %d to start before the queue filled", index)
		}
	}

	startedMu.Lock()
	if startedCount != maxPendingStreamRequests {
		startedMu.Unlock()
		t.Fatalf("expected exactly %d started requests before backpressure, got %d", maxPendingStreamRequests, startedCount)
	}
	startedMu.Unlock()

	extraRequestDone := make(chan error, 1)
	go func() {
		extraRequestDone <- encoder.Encode(requestEnvelope{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"req-blocking-overflow"`),
			Method:  "test.blocking",
			Params:  mustMarshal(t, map[string]any{}),
		})
	}()

	select {
	case <-startedSignals:
		t.Fatal("expected overflow request to wait until a pending slot is released")
	case <-time.After(250 * time.Millisecond):
	}

	close(releaseBlocking)
	releasedBlocking = true

	select {
	case <-startedSignals:
	case <-time.After(2 * time.Second):
		t.Fatal("expected overflow request to start after pending capacity became available")
	}

	select {
	case err := <-extraRequestDone:
		if err != nil {
			t.Fatalf("encode overflow request: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected overflow request write to complete after backpressure released")
	}
}

func TestHandleStreamConnDropsOverflowRequestAfterDisconnectWithFullPendingQueue(t *testing.T) {
	server := newTestServer()
	startedSignals := make(chan struct{}, maxPendingStreamRequests+1)
	releaseBlocking := make(chan struct{})
	releasedBlocking := false
	defer func() {
		if !releasedBlocking {
			close(releaseBlocking)
		}
	}()

	var startedMu sync.Mutex
	startedCount := 0
	server.handlers["test.blocking"] = func(_ map[string]any) (any, *rpcError) {
		startedMu.Lock()
		startedCount++
		startedMu.Unlock()

		startedSignals <- struct{}{}
		<-releaseBlocking
		return map[string]any{"status": "released"}, nil
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen loopback: %v", err)
	}
	defer listener.Close()

	acceptDone := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			acceptDone <- err
			return
		}
		server.handleStreamConn(conn)
		acceptDone <- nil
	}()

	right, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial loopback: %v", err)
	}

	encoder := json.NewEncoder(right)
	for index := 0; index < maxPendingStreamRequests; index++ {
		request := requestEnvelope{
			JSONRPC: "2.0",
			ID:      json.RawMessage(fmt.Sprintf(`"req-disconnect-blocking-%d"`, index)),
			Method:  "test.blocking",
			Params:  mustMarshal(t, map[string]any{}),
		}
		if err := encoder.Encode(request); err != nil {
			t.Fatalf("encode blocking request %d: %v", index, err)
		}
	}

	for index := 0; index < maxPendingStreamRequests; index++ {
		select {
		case <-startedSignals:
		case <-time.After(2 * time.Second):
			t.Fatalf("expected request %d to start before the queue filled", index)
		}
	}

	startedMu.Lock()
	if startedCount != maxPendingStreamRequests {
		startedMu.Unlock()
		t.Fatalf("expected exactly %d started requests before the disconnect race, got %d", maxPendingStreamRequests, startedCount)
	}
	startedMu.Unlock()

	extraRequestDone := make(chan error, 1)
	go func() {
		extraRequestDone <- encoder.Encode(requestEnvelope{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"req-disconnect-overflow"`),
			Method:  "test.blocking",
			Params:  mustMarshal(t, map[string]any{}),
		})
	}()

	select {
	case <-startedSignals:
		t.Fatal("expected overflow request to remain queued before disconnect")
	case <-time.After(250 * time.Millisecond):
	}

	if err := right.Close(); err != nil {
		t.Fatalf("close client stream: %v", err)
	}

	select {
	case <-extraRequestDone:
	case <-time.After(2 * time.Second):
		t.Fatal("expected overflow request write to exit after client disconnect")
	}

	close(releaseBlocking)
	releasedBlocking = true

	select {
	case <-startedSignals:
		t.Fatal("expected disconnected overflow request not to start after pending capacity frees")
	case <-time.After(300 * time.Millisecond):
	}

	startedMu.Lock()
	if startedCount != maxPendingStreamRequests {
		startedMu.Unlock()
		t.Fatalf("expected disconnect to prevent stale overflow dispatch, got %d calls", startedCount)
	}
	startedMu.Unlock()

	select {
	case err := <-acceptDone:
		if err != nil && !errors.Is(err, net.ErrClosed) {
			t.Fatalf("accept loopback: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected loopback stream to shut down after pending workers released")
	}
}

func TestHandleStreamConnDropsDecodedSameTaskBacklogAfterDisconnect(t *testing.T) {
	server := newTestServer()
	taskID := "task_disconnect_same_task_backlog"
	startedSignals := make(chan int, maxPendingStreamRequests)
	releaseFirst := make(chan struct{})

	var startedMu sync.Mutex
	startedCount := 0
	server.handlers["test.same.task.blocking"] = func(params map[string]any) (any, *rpcError) {
		startedMu.Lock()
		startedCount++
		callIndex := startedCount
		startedMu.Unlock()

		startedSignals <- callIndex
		if callIndex == 1 {
			<-releaseFirst
		}

		return map[string]any{
			"task": map[string]any{
				"task_id": stringValue(params, "task_id", ""),
			},
		}, nil
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen loopback: %v", err)
	}
	defer listener.Close()

	acceptDone := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			acceptDone <- err
			return
		}
		server.handleStreamConn(conn)
		acceptDone <- nil
	}()

	right, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial loopback: %v", err)
	}

	encoder := json.NewEncoder(right)
	for index := 0; index < maxPendingStreamRequests; index++ {
		request := requestEnvelope{
			JSONRPC: "2.0",
			ID:      json.RawMessage(fmt.Sprintf(`"req-same-task-disconnect-%d"`, index)),
			Method:  "test.same.task.blocking",
			Params: mustMarshal(t, map[string]any{
				"task_id": taskID,
			}),
		}
		if err := encoder.Encode(request); err != nil {
			t.Fatalf("encode same-task request %d: %v", index, err)
		}
	}

	select {
	case callIndex := <-startedSignals:
		if callIndex != 1 {
			t.Fatalf("expected the first same-task request to start first, got call %d", callIndex)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected the first same-task request to start")
	}

	select {
	case callIndex := <-startedSignals:
		t.Fatalf("expected same-task backlog to stay queued behind the first request, got call %d", callIndex)
	case <-time.After(250 * time.Millisecond):
	}

	if err := right.Close(); err != nil {
		t.Fatalf("close client stream: %v", err)
	}

	close(releaseFirst)

	select {
	case callIndex := <-startedSignals:
		t.Fatalf("expected disconnected same-task backlog not to start after release, got call %d", callIndex)
	case <-time.After(500 * time.Millisecond):
	}

	startedMu.Lock()
	if startedCount != 1 {
		startedMu.Unlock()
		t.Fatalf("expected only the first same-task request to dispatch before disconnect, got %d", startedCount)
	}
	startedMu.Unlock()

	select {
	case err := <-acceptDone:
		if err != nil && !errors.Is(err, net.ErrClosed) {
			t.Fatalf("accept loopback: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected loopback stream to shut down after same-task backlog release")
	}
}

func TestHandleStreamConnKeepsHealthyIdleSameTaskBacklogAlive(t *testing.T) {
	server := newTestServer()
	taskID := "task_idle_same_task_backlog"
	startedSignals := make(chan int, maxPendingStreamRequests)
	releaseFirst := make(chan struct{})

	var startedMu sync.Mutex
	startedCount := 0
	server.handlers["test.same.task.healthy"] = func(params map[string]any) (any, *rpcError) {
		startedMu.Lock()
		startedCount++
		callIndex := startedCount
		startedMu.Unlock()

		startedSignals <- callIndex
		if callIndex == 1 {
			<-releaseFirst
		}

		return map[string]any{
			"task": map[string]any{
				"task_id": stringValue(params, "task_id", ""),
			},
		}, nil
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen loopback: %v", err)
	}
	defer listener.Close()

	acceptDone := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			acceptDone <- err
			return
		}
		server.handleStreamConn(conn)
		acceptDone <- nil
	}()

	right, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial loopback: %v", err)
	}
	defer right.Close()

	encoder := json.NewEncoder(right)
	decoder := json.NewDecoder(right)
	for index := 0; index < maxPendingStreamRequests; index++ {
		request := requestEnvelope{
			JSONRPC: "2.0",
			ID:      json.RawMessage(fmt.Sprintf(`"req-same-task-healthy-%d"`, index)),
			Method:  "test.same.task.healthy",
			Params: mustMarshal(t, map[string]any{
				"task_id": taskID,
			}),
		}
		if err := encoder.Encode(request); err != nil {
			t.Fatalf("encode same-task request %d: %v", index, err)
		}
	}

	select {
	case callIndex := <-startedSignals:
		if callIndex != 1 {
			t.Fatalf("expected the first same-task request to start first, got call %d", callIndex)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected the first same-task request to start")
	}

	select {
	case callIndex := <-startedSignals:
		t.Fatalf("expected same-task backlog to stay queued behind the first request, got call %d", callIndex)
	case <-time.After(250 * time.Millisecond):
	}

	close(releaseFirst)

	if err := right.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	defer func() {
		if err := right.SetReadDeadline(time.Time{}); err != nil {
			t.Fatalf("clear read deadline: %v", err)
		}
	}()

	var firstEnvelope map[string]any
	if err := decoder.Decode(&firstEnvelope); err != nil {
		t.Fatalf("decode first same-task response: %v", err)
	}
	if firstEnvelope["id"] == nil {
		t.Fatalf("expected the first same-task response envelope, got %+v", firstEnvelope)
	}
	if firstEnvelope["error"] != nil {
		t.Fatalf("expected first same-task response to succeed, got %+v", firstEnvelope)
	}

	select {
	case callIndex := <-startedSignals:
		if callIndex != 2 {
			t.Fatalf("expected the second same-task request to dispatch next, got call %d", callIndex)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected the second same-task request to dispatch after the first response")
	}

	var secondEnvelope map[string]any
	if err := decoder.Decode(&secondEnvelope); err != nil {
		t.Fatalf("decode second same-task response: %v", err)
	}
	if secondEnvelope["id"] == nil {
		t.Fatalf("expected the second same-task response envelope, got %+v", secondEnvelope)
	}
	if secondEnvelope["error"] != nil {
		t.Fatalf("expected same-task backlog to stay on the healthy shared stream, got %+v", secondEnvelope)
	}

	select {
	case err := <-acceptDone:
		if err != nil && !errors.Is(err, net.ErrClosed) {
			t.Fatalf("accept loopback: %v", err)
		}
	case <-time.After(250 * time.Millisecond):
	}
}
