package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
)

type stubMainApp struct {
	startErr error
	started  bool
	ctx      context.Context
}

func (s *stubMainApp) Start(ctx context.Context) error {
	s.started = true
	s.ctx = ctx
	return s.startErr
}

func TestRunPassesFlagsToBootstrapAndStartsApp(t *testing.T) {
	originalLoadConfig := loadConfigForMain
	originalNewBootstrap := newBootstrapForMain
	originalLogPrintf := logPrintfForMain
	t.Cleanup(func() {
		loadConfigForMain = originalLoadConfig
		newBootstrapForMain = originalNewBootstrap
		logPrintfForMain = originalLogPrintf
	})

	dataDir := t.TempDir()
	pipeName := `\\.\pipe\cialloclaw-rpc-main-test`
	debugHTTP := `127.0.0.1:0`
	var capturedOptions config.LoadOptions
	var capturedConfig config.Config
	var loggedMessage string
	app := &stubMainApp{}

	loadConfigForMain = func(options config.LoadOptions) config.Config {
		capturedOptions = options
		return config.Load(options)
	}
	newBootstrapForMain = func(cfg config.Config) (appStarter, error) {
		capturedConfig = cfg
		return app, nil
	}
	logPrintfForMain = func(format string, args ...any) {
		loggedMessage = fmt.Sprintf(format, args...)
	}

	ctx := context.WithValue(context.Background(), struct{}{}, "main-run")
	err := run(ctx, []string{
		"--data-dir", dataDir,
		"--named-pipe", pipeName,
		"--debug-http", debugHTTP,
	})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if capturedOptions.DataDir != dataDir {
		t.Fatalf("expected data-dir %q, got %q", dataDir, capturedOptions.DataDir)
	}
	if capturedOptions.NamedPipeName != pipeName {
		t.Fatalf("expected named pipe %q, got %q", pipeName, capturedOptions.NamedPipeName)
	}
	if capturedOptions.DebugHTTPAddress != debugHTTP {
		t.Fatalf("expected debug http %q, got %q", debugHTTP, capturedOptions.DebugHTTPAddress)
	}
	if capturedConfig.DataDir != dataDir {
		t.Fatalf("expected loaded config data dir %q, got %q", dataDir, capturedConfig.DataDir)
	}
	if capturedConfig.RPC.NamedPipeName != pipeName {
		t.Fatalf("expected loaded config pipe %q, got %q", pipeName, capturedConfig.RPC.NamedPipeName)
	}
	if capturedConfig.RPC.DebugHTTPAddress != debugHTTP {
		t.Fatalf("expected loaded config debug http %q, got %q", debugHTTP, capturedConfig.RPC.DebugHTTPAddress)
	}
	if !app.started {
		t.Fatal("expected bootstrap app to start")
	}
	if app.ctx != ctx {
		t.Fatal("expected run to pass the original context into app.Start")
	}
	if !strings.Contains(loggedMessage, pipeName) || !strings.Contains(loggedMessage, dataDir) {
		t.Fatalf("expected startup log to include runtime paths, got %q", loggedMessage)
	}
}

func TestRunWrapsBootstrapError(t *testing.T) {
	originalLoadConfig := loadConfigForMain
	originalNewBootstrap := newBootstrapForMain
	originalLogPrintf := logPrintfForMain
	t.Cleanup(func() {
		loadConfigForMain = originalLoadConfig
		newBootstrapForMain = originalNewBootstrap
		logPrintfForMain = originalLogPrintf
	})

	loadConfigForMain = func(options config.LoadOptions) config.Config {
		return config.Load(options)
	}
	newBootstrapForMain = func(config.Config) (appStarter, error) {
		return nil, errors.New("bootstrap failed")
	}
	logPrintfForMain = func(string, ...any) {}

	err := run(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "bootstrap local service: bootstrap failed") {
		t.Fatalf("expected wrapped bootstrap error, got %v", err)
	}
}

func TestRunWrapsStartError(t *testing.T) {
	originalLoadConfig := loadConfigForMain
	originalNewBootstrap := newBootstrapForMain
	originalLogPrintf := logPrintfForMain
	t.Cleanup(func() {
		loadConfigForMain = originalLoadConfig
		newBootstrapForMain = originalNewBootstrap
		logPrintfForMain = originalLogPrintf
	})

	loadConfigForMain = func(options config.LoadOptions) config.Config {
		return config.Load(options)
	}
	newBootstrapForMain = func(config.Config) (appStarter, error) {
		return &stubMainApp{startErr: errors.New("start failed")}, nil
	}
	logPrintfForMain = func(string, ...any) {}

	err := run(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "run local service: start failed") {
		t.Fatalf("expected wrapped start error, got %v", err)
	}
}

func TestRunReturnsFlagParseError(t *testing.T) {
	err := run(context.Background(), []string{"--unknown-flag"})
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("expected flag parse error, got %v", err)
	}
}

func TestMainInvokesRunWithProcessArgs(t *testing.T) {
	originalNotifyContext := notifyContextForMain
	originalRunMain := runMainForProcess
	originalLogFatal := logFatalForMain
	originalArgs := os.Args
	t.Cleanup(func() {
		notifyContextForMain = originalNotifyContext
		runMainForProcess = originalRunMain
		logFatalForMain = originalLogFatal
		os.Args = originalArgs
	})

	expectedArgs := []string{"--data-dir", t.TempDir(), "--debug-http", "127.0.0.1:0"}
	os.Args = append([]string{"local-service"}, expectedArgs...)
	ctxFromMain := context.WithValue(context.Background(), struct{}{}, "main")
	stopCalled := false
	runCalled := false

	notifyContextForMain = func(context.Context, ...os.Signal) (context.Context, context.CancelFunc) {
		return ctxFromMain, func() {
			stopCalled = true
		}
	}
	runMainForProcess = func(ctx context.Context, args []string) error {
		runCalled = true
		if ctx != ctxFromMain {
			t.Fatalf("expected main to pass the notified context into run, got %v", ctx)
		}
		if !reflect.DeepEqual(args, expectedArgs) {
			t.Fatalf("expected main to forward process args %v, got %v", expectedArgs, args)
		}
		return nil
	}
	logFatalForMain = func(...any) {
		t.Fatal("did not expect main to call log.Fatal on successful run")
	}

	main()

	if !runCalled {
		t.Fatal("expected main to invoke run")
	}
	if !stopCalled {
		t.Fatal("expected main to call the signal cleanup function")
	}
}

func TestMainLogsFatalWhenRunFails(t *testing.T) {
	originalNotifyContext := notifyContextForMain
	originalRunMain := runMainForProcess
	originalLogFatal := logFatalForMain
	originalArgs := os.Args
	t.Cleanup(func() {
		notifyContextForMain = originalNotifyContext
		runMainForProcess = originalRunMain
		logFatalForMain = originalLogFatal
		os.Args = originalArgs
	})

	os.Args = []string{"local-service", "--named-pipe", `\\.\pipe\main-fatal-test`}
	stopCalled := false
	runErr := errors.New("run failed")
	var fatalArgs []any

	notifyContextForMain = func(context.Context, ...os.Signal) (context.Context, context.CancelFunc) {
		return context.Background(), func() {
			stopCalled = true
		}
	}
	runMainForProcess = func(context.Context, []string) error {
		return runErr
	}
	logFatalForMain = func(args ...any) {
		fatalArgs = append([]any(nil), args...)
	}

	main()

	if !stopCalled {
		t.Fatal("expected main to call the signal cleanup function after a run failure")
	}
	if len(fatalArgs) != 1 {
		t.Fatalf("expected main to call log.Fatal with one argument, got %v", fatalArgs)
	}
	if fatalArgs[0] != runErr {
		t.Fatalf("expected main to forward the run error to log.Fatal, got %v", fatalArgs[0])
	}
}
