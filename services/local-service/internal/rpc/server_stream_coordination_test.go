// RPC stream coordination tests verify task ownership, serialization, and replay order.
package rpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
)

func TestHandleStreamConnSerializesTaskStartingRequestsOnSharedConnection(t *testing.T) {
	testCases := []struct {
		name         string
		method       string
		firstParams  map[string]any
		secondParams map[string]any
	}{
		{
			name:   "input submit",
			method: "agent.input.submit",
			firstParams: map[string]any{
				"request_meta": rpcRequestMeta("trace_serialized_submit_first"),
				"session_id":   "sess_serialized_submit",
				"source":       "floating_ball",
				"trigger":      "hover_text_input",
				"input": map[string]any{
					"type":       "text",
					"text":       "first submit",
					"input_mode": "text",
				},
				"context": map[string]any{},
			},
			secondParams: map[string]any{
				"request_meta": map[string]any{
					"trace_id":    "trace_serialized_submit_second",
					"client_time": "2026-05-10T10:00:01Z",
				},
				"session_id": "sess_serialized_submit",
				"source":     "floating_ball",
				"trigger":    "hover_text_input",
				"input": map[string]any{
					"type":       "text",
					"text":       "second submit",
					"input_mode": "text",
				},
				"context": map[string]any{},
			},
		},
		{
			name:   "task start",
			method: "agent.task.start",
			firstParams: map[string]any{
				"request_meta": rpcRequestMeta("trace_serialized_start_first"),
				"session_id":   "sess_serialized_start",
				"source":       "floating_ball",
				"trigger":      "text_selected_click",
				"input": map[string]any{
					"type": "text_selection",
					"text": "first selection",
				},
			},
			secondParams: map[string]any{
				"request_meta": map[string]any{
					"trace_id":    "trace_serialized_start_second",
					"client_time": "2026-05-10T10:00:01Z",
				},
				"session_id": "sess_serialized_start",
				"source":     "floating_ball",
				"trigger":    "text_selected_click",
				"input": map[string]any{
					"type": "text_selection",
					"text": "second selection",
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			server := newTestServer()
			firstStarted := make(chan struct{})
			releaseFirst := make(chan struct{})
			var callCount int
			var callMu sync.Mutex

			server.handlers[testCase.method] = func(_ map[string]any) (any, *rpcError) {
				callMu.Lock()
				callCount++
				currentCall := callCount
				callMu.Unlock()

				if currentCall == 1 {
					select {
					case <-firstStarted:
					default:
						close(firstStarted)
					}
					<-releaseFirst
				}

				return map[string]any{
					"task": map[string]any{
						"task_id": fmt.Sprintf("task_serial_%d", currentCall),
					},
				}, nil
			}

			left, right := net.Pipe()
			defer left.Close()
			defer right.Close()

			go server.handleStreamConn(left)

			encoder := json.NewEncoder(right)
			decoder := json.NewDecoder(right)
			firstRequest := requestEnvelope{
				JSONRPC: "2.0",
				ID:      json.RawMessage(`"req-task-starting-1"`),
				Method:  testCase.method,
				Params:  mustMarshal(t, testCase.firstParams),
			}
			secondRequest := requestEnvelope{
				JSONRPC: "2.0",
				ID:      json.RawMessage(`"req-task-starting-2"`),
				Method:  testCase.method,
				Params:  mustMarshal(t, testCase.secondParams),
			}

			if err := encoder.Encode(firstRequest); err != nil {
				t.Fatalf("encode first request: %v", err)
			}

			select {
			case <-firstStarted:
			case <-time.After(500 * time.Millisecond):
				t.Fatal("expected first task-starting request to begin running")
			}

			if err := encoder.Encode(secondRequest); err != nil {
				t.Fatalf("encode second request: %v", err)
			}

			type decodeResult struct {
				envelope map[string]any
				err      error
			}
			firstResponseCh := make(chan decodeResult, 1)
			go func() {
				var envelope map[string]any
				err := decoder.Decode(&envelope)
				firstResponseCh <- decodeResult{envelope: envelope, err: err}
			}()

			select {
			case result := <-firstResponseCh:
				if result.err != nil {
					t.Fatalf("expected no response before the first task-starting request finishes, got %v", result.err)
				}
				t.Fatalf("expected second task-starting request to stay queued until the first finishes, got %+v", result.envelope)
			case <-time.After(250 * time.Millisecond):
			}

			close(releaseFirst)

			seenResponses := map[string]bool{}
			select {
			case result := <-firstResponseCh:
				if result.err != nil {
					t.Fatalf("decode first serialized response envelope: %v", result.err)
				}
				id, _ := result.envelope["id"].(string)
				if id == "req-task-starting-1" || id == "req-task-starting-2" {
					seenResponses[id] = true
				}
			case <-time.After(1 * time.Second):
				t.Fatal("expected first queued task-starting request to finish after release")
			}

			if err := right.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
				t.Fatalf("set response deadline: %v", err)
			}
			for len(seenResponses) < 2 {
				var envelope map[string]any
				if err := decoder.Decode(&envelope); err != nil {
					t.Fatalf("decode serialized response envelope: %v", err)
				}
				id, _ := envelope["id"].(string)
				if id == "req-task-starting-1" || id == "req-task-starting-2" {
					seenResponses[id] = true
				}
			}
			if err := right.SetReadDeadline(time.Time{}); err != nil {
				t.Fatalf("clear response deadline: %v", err)
			}
		})
	}
}

func TestHandleStreamConnReplaysLateTaskNotificationsBeforeQueuedSameTaskFollowUp(t *testing.T) {
	server := newTestServer()
	startResult, err := startTaskForTest(server.orchestrator, map[string]any{
		"session_id": "sess_late_task_replay",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "queue notifications for shared stream replay",
		},
	})
	if err != nil {
		t.Fatalf("seed task.start: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	notifications, err := server.orchestrator.PendingNotifications(taskID)
	if err != nil || len(notifications) == 0 {
		t.Fatalf("expected seeded task to queue notifications, err=%v notifications=%+v", err, notifications)
	}

	firstReturned := make(chan struct{})
	server.handlers["agent.task.start"] = func(_ map[string]any) (any, *rpcError) {
		select {
		case <-firstReturned:
		default:
			close(firstReturned)
		}
		return map[string]any{
			"task": map[string]any{
				"task_id": taskID,
			},
		}, nil
	}
	server.handlers["test.followup.task"] = func(params map[string]any) (any, *rpcError) {
		return map[string]any{
			"task": map[string]any{
				"task_id": stringValue(params, "task_id", ""),
			},
		}, nil
	}

	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	go server.handleStreamConn(left)

	encoder := json.NewEncoder(right)
	decoder := json.NewDecoder(right)
	firstRequest := requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-late-task-starter"`),
		Method:  "agent.task.start",
		Params: mustMarshal(t, map[string]any{
			"request_meta": rpcRequestMeta("trace_late_task_starter"),
			"session_id":   "sess_late_task_response_owner",
			"source":       "floating_ball",
			"trigger":      "text_selected_click",
			"input": map[string]any{
				"type": "text_selection",
				"text": "start a task on the shared stream",
			},
		}),
	}
	if err := encoder.Encode(firstRequest); err != nil {
		t.Fatalf("encode first late-task response request: %v", err)
	}

	select {
	case <-firstReturned:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected first task-starting request to finish dispatch")
	}

	secondRequest := requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-late-task-followup"`),
		Method:  "test.followup.task",
		Params: mustMarshal(t, map[string]any{
			"task_id": taskID,
		}),
	}
	if err := encoder.Encode(secondRequest); err != nil {
		t.Fatalf("encode same-task follow-up request: %v", err)
	}

	if err := right.SetReadDeadline(time.Now().Add(1500 * time.Millisecond)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	defer func() {
		if err := right.SetReadDeadline(time.Time{}); err != nil {
			t.Fatalf("clear read deadline: %v", err)
		}
	}()

	var firstEnvelope map[string]any
	if err := decoder.Decode(&firstEnvelope); err != nil {
		t.Fatalf("decode first response envelope: %v", err)
	}
	if firstID, _ := firstEnvelope["id"].(string); firstID != "req-late-task-starter" {
		t.Fatalf("expected first envelope to be the starter response, got %+v", firstEnvelope)
	}

	notificationBeforeFollowUp := false
	followUpResponseSeen := false
	for index := 0; index < 12; index++ {
		var envelope map[string]any
		if err := decoder.Decode(&envelope); err != nil {
			t.Fatalf("decode shared stream envelope: %v", err)
		}
		if envelopeID, _ := envelope["id"].(string); envelopeID == "req-late-task-followup" {
			followUpResponseSeen = true
			break
		}
		if method, _ := envelope["method"].(string); method != "" {
			notificationBeforeFollowUp = true
		}
	}
	if !followUpResponseSeen {
		t.Fatal("expected follow-up response to arrive on the shared stream")
	}
	if !notificationBeforeFollowUp {
		t.Fatal("expected buffered notifications for the started task to replay before the queued same-task follow-up response")
	}
}

func TestHandleStreamConnTaskListDoesNotStealBufferedNotifications(t *testing.T) {
	server := newTestServer()
	startResult, err := startTaskForTest(server.orchestrator, map[string]any{
		"session_id": "sess_task_list_replay_owner",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "queue notifications for task.list replay ownership",
		},
	})
	if err != nil {
		t.Fatalf("seed task.start: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	notifications, err := server.orchestrator.PendingNotifications(taskID)
	if err != nil || len(notifications) == 0 {
		t.Fatalf("expected seeded task to queue notifications, err=%v notifications=%+v", err, notifications)
	}

	listStarted := make(chan struct{})
	allowListReturn := make(chan struct{})
	server.handlers["agent.task.list"] = func(_ map[string]any) (any, *rpcError) {
		select {
		case <-listStarted:
		default:
			close(listStarted)
		}
		<-allowListReturn
		return map[string]any{
			"items": []any{
				map[string]any{"task_id": taskID},
			},
		}, nil
	}
	server.handlers["test.followup.task"] = func(params map[string]any) (any, *rpcError) {
		return map[string]any{
			"task": map[string]any{
				"task_id": stringValue(params, "task_id", ""),
			},
		}, nil
	}

	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	go server.handleStreamConn(left)

	encoder := json.NewEncoder(right)
	decoder := json.NewDecoder(right)
	if err := encoder.Encode(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-task-list-owned"`),
		Method:  "agent.task.list",
		Params: mustMarshal(t, map[string]any{
			"request_meta": rpcRequestMeta("trace_task_list_owned"),
		}),
	}); err != nil {
		t.Fatalf("encode task.list request: %v", err)
	}

	select {
	case <-listStarted:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected task.list request to start dispatch")
	}

	if err := encoder.Encode(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-task-list-followup"`),
		Method:  "test.followup.task",
		Params: mustMarshal(t, map[string]any{
			"task_id": taskID,
		}),
	}); err != nil {
		t.Fatalf("encode follow-up request: %v", err)
	}
	close(allowListReturn)

	if err := right.SetReadDeadline(time.Now().Add(1500 * time.Millisecond)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	defer func() {
		if err := right.SetReadDeadline(time.Time{}); err != nil {
			t.Fatalf("clear read deadline: %v", err)
		}
	}()

	taskListResponseSeen := false
	followUpResponseSeen := false
	notificationAfterFollowUp := false
	for index := 0; index < 16; index++ {
		var envelope map[string]any
		if err := decoder.Decode(&envelope); err != nil {
			t.Fatalf("decode shared stream envelope: %v", err)
		}
		if envelopeID, _ := envelope["id"].(string); envelopeID == "req-task-list-owned" {
			taskListResponseSeen = true
		} else if envelopeID, _ := envelope["id"].(string); envelopeID == "req-task-list-followup" {
			followUpResponseSeen = true
		} else if method, _ := envelope["method"].(string); method != "" {
			if !followUpResponseSeen {
				t.Fatalf("expected task.list not to replay %s before the follow-up response, got %+v", method, envelope)
			}
			notificationAfterFollowUp = true
		}
		if taskListResponseSeen && followUpResponseSeen && notificationAfterFollowUp {
			break
		}
	}
	if !taskListResponseSeen {
		t.Fatal("expected task.list response to arrive on the shared stream")
	}
	if !followUpResponseSeen {
		t.Fatal("expected follow-up response to arrive on the shared stream")
	}
	if !notificationAfterFollowUp {
		t.Fatal("expected the owning follow-up request to replay buffered notifications after its response")
	}
}

func TestStreamTaskCoordinatorReleasesIdleTaskLocks(t *testing.T) {
	coordinator := newStreamTaskCoordinator()
	coordinator.withTaskLocks(map[string]bool{
		"task_cleanup": true,
	}, func() {})

	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	if len(coordinator.locks) != 0 {
		t.Fatalf("expected idle task locks to be released, got %+v", coordinator.locks)
	}
}

func TestHandleStreamConnKeepsQueuedReadsResponsiveWhileLoopTaskRuns(t *testing.T) {
	modelClient := &selectiveWaitLoopModelClient{
		stubLoopModelClient: stubLoopModelClient{
			toolResult: model.ToolCallResult{
				OutputText: "Concurrent stream finished.",
			},
			generateToolWait: make(chan struct{}),
			generateToolSeen: make(chan struct{}),
		},
	}
	server := newTestServerWithModelClient(modelClient)

	startTask := func(sessionID string) string {
		t.Helper()
		result, err := startTaskForTest(server.orchestrator, map[string]any{
			"session_id": sessionID,
			"source":     "floating_ball",
			"trigger":    "text_selected_click",
			"input": map[string]any{
				"type": "text_selection",
				"text": "inspect this workspace",
			},
		})
		if err != nil {
			t.Fatalf("seed task.start for %s: %v", sessionID, err)
		}
		return result["task"].(map[string]any)["task_id"].(string)
	}

	taskA := startTask("sess_pipe_queue_a")
	taskB := startTask("sess_pipe_queue_b")
	modelClient.blockedTaskID = taskA

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
	decoder := json.NewDecoder(right)
	confirmRequest := requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-loop-blocked"`),
		Method:  "agent.task.confirm",
		Params: mustMarshal(t, map[string]any{
			"request_meta": rpcRequestMeta("trace_task_confirm_loop_blocked"),
			"task_id":      taskA,
			"confirmed":    false,
			"corrected_intent": map[string]any{
				"name":      "agent_loop",
				"arguments": map[string]any{},
			},
		}),
	}
	if err := encoder.Encode(confirmRequest); err != nil {
		t.Fatalf("encode blocked confirm request: %v", err)
	}

	select {
	case <-modelClient.generateToolSeen:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected blocked loop task to start model execution")
	}

	detailRequest := requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-task-detail-queued"`),
		Method:  "agent.task.detail.get",
		Params: mustMarshal(t, map[string]any{
			"request_meta": rpcRequestMeta("trace_task_detail_loop_blocked"),
			"task_id":      taskB,
		}),
	}
	if err := encoder.Encode(detailRequest); err != nil {
		t.Fatalf("encode queued detail request: %v", err)
	}

	if err := right.SetReadDeadline(time.Now().Add(750 * time.Millisecond)); err != nil {
		t.Fatalf("set queued response deadline: %v", err)
	}
	defer func() {
		if err := right.SetReadDeadline(time.Time{}); err != nil {
			t.Fatalf("clear queued response deadline: %v", err)
		}
	}()

	queuedResponseSeen := false
	for index := 0; index < 12; index++ {
		var envelope map[string]any
		if err := decoder.Decode(&envelope); err != nil {
			break
		}
		responseID, _ := envelope["id"].(string)
		if responseID != "req-task-detail-queued" {
			continue
		}
		result, ok := envelope["result"].(map[string]any)
		if !ok {
			t.Fatalf("expected queued detail success envelope, got %+v", envelope)
		}
		data, ok := result["data"].(map[string]any)
		if !ok {
			t.Fatalf("expected queued detail response payload, got %+v", envelope)
		}
		task, ok := data["task"].(map[string]any)
		if !ok || task["task_id"] != taskB {
			t.Fatalf("expected queued detail response for %s, got %+v", taskB, envelope)
		}
		queuedResponseSeen = true
		break
	}

	close(modelClient.generateToolWait)

	if !queuedResponseSeen {
		t.Fatal("expected queued task detail request to complete before the blocked loop response")
	}
}

func TestHandleStreamConnSerializesConcurrentRequestsForSameTask(t *testing.T) {
	modelClient := &selectiveWaitLoopModelClient{
		stubLoopModelClient: stubLoopModelClient{
			toolResult: model.ToolCallResult{
				OutputText: "Same-task stream finished.",
			},
			generateToolWait: make(chan struct{}),
			generateToolSeen: make(chan struct{}),
		},
	}
	server := newTestServerWithModelClient(modelClient)

	result, err := startTaskForTest(server.orchestrator, map[string]any{
		"session_id": "sess_pipe_same_task",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "inspect this workspace",
		},
	})
	if err != nil {
		t.Fatalf("seed task.start: %v", err)
	}
	taskID := result["task"].(map[string]any)["task_id"].(string)
	modelClient.blockedTaskID = taskID

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
	decoder := json.NewDecoder(right)
	envelopeCh := make(chan map[string]any, 32)
	decodeErrCh := make(chan error, 1)
	go func() {
		for {
			var envelope map[string]any
			if err := decoder.Decode(&envelope); err != nil {
				decodeErrCh <- err
				return
			}
			envelopeCh <- envelope
		}
	}()
	confirmRequest := requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-loop-same-task"`),
		Method:  "agent.task.confirm",
		Params: mustMarshal(t, map[string]any{
			"request_meta": rpcRequestMeta("trace_task_confirm_same_task"),
			"task_id":      taskID,
			"confirmed":    false,
			"corrected_intent": map[string]any{
				"name":      "agent_loop",
				"arguments": map[string]any{},
			},
		}),
	}
	if err := encoder.Encode(confirmRequest); err != nil {
		t.Fatalf("encode blocked confirm request: %v", err)
	}

	select {
	case <-modelClient.generateToolSeen:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected blocked same-task loop to start model execution")
	}

	detailRequest := requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-task-detail-same-task"`),
		Method:  "agent.task.detail.get",
		Params: mustMarshal(t, map[string]any{
			"request_meta": rpcRequestMeta("trace_task_detail_same_task"),
			"task_id":      taskID,
		}),
	}
	if err := encoder.Encode(detailRequest); err != nil {
		t.Fatalf("encode same-task detail request: %v", err)
	}

	detailResponseSeenEarly := false
	earlyWindow := time.After(250 * time.Millisecond)
	for !detailResponseSeenEarly {
		select {
		case envelope := <-envelopeCh:
			responseID, _ := envelope["id"].(string)
			if responseID == "req-task-detail-same-task" {
				detailResponseSeenEarly = true
			}
		case err := <-decodeErrCh:
			t.Fatalf("decode same-task early envelope: %v", err)
		case <-earlyWindow:
			goto afterEarlyWindow
		}
	}

afterEarlyWindow:

	if detailResponseSeenEarly {
		t.Fatal("expected same-task detail request to wait until the blocked loop request finishes")
	}

	close(modelClient.generateToolWait)
	detailResponseSeen := false
	postUnblockDeadline := time.After(3 * time.Second)
	for !detailResponseSeen {
		select {
		case envelope := <-envelopeCh:
			responseID, _ := envelope["id"].(string)
			if responseID == "req-task-detail-same-task" {
				detailResponseSeen = true
			}
		case err := <-decodeErrCh:
			t.Fatalf("decode same-task post-unblock envelope: %v", err)
		case <-postUnblockDeadline:
			t.Fatal("expected same-task detail response after the blocked loop request completed")
		}
	}
}
