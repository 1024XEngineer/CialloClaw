package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "[launcher error]", err)
		fmt.Fprintln(os.Stderr, "Press Enter to exit...")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		os.Exit(1)
	}
}

func run() error {
	repoRoot, err := executableDir()
	if err != nil {
		return err
	}

	goExe := `E:\go\go1.24.13.windows-amd64\go\bin\go.exe`
	if _, err := os.Stat(goExe); err != nil {
		return fmt.Errorf("Go not found at %s", goExe)
	}

	qtPrefix, err := findQtPrefix()
	if err != nil {
		return err
	}

	backendDir := filepath.Join(repoRoot, "backend")
	frontendQmlDir := filepath.Join(repoRoot, "frontend", "qml")
	backendExe := filepath.Join(backendDir, "backend.exe")
	qmlExe := filepath.Join(qtPrefix, "bin", "qml.exe")
	if _, err := os.Stat(qmlExe); err != nil {
		return fmt.Errorf("qml.exe not found at %s", qmlExe)
	}

	if err := buildBackend(goExe, backendDir, backendExe); err != nil {
		return err
	}

	backendCmd, err := startProcess(backendExe, backendDir, nil)
	if err != nil {
		return err
	}
	defer killProcess(backendCmd)

	if err := waitBackend("http://127.0.0.1:17888/api/bootstrap", 20*time.Second); err != nil {
		return err
	}

	qmlEnv := append(os.Environ(),
		"PATH="+qtPrefix+string(os.PathListSeparator)+filepath.Join(qtPrefix, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"),
		"QT_PLUGIN_PATH="+filepath.Join(qtPrefix, "plugins"),
		"QML2_IMPORT_PATH="+filepath.Join(qtPrefix, "qml")+string(os.PathListSeparator)+frontendQmlDir,
		"QML_IMPORT_PATH="+filepath.Join(qtPrefix, "qml")+string(os.PathListSeparator)+frontendQmlDir,
	)

	qmlCmd, err := startProcess(qmlExe, frontendQmlDir, qmlEnv, "Main.qml")
	if err != nil {
		return err
	}
	defer killProcess(qmlCmd)

	fmt.Println("CialloClaw Prototype 01 started.")
	fmt.Println("Backend: http://127.0.0.1:17888")
	if err := qmlCmd.Wait(); err != nil {
		return fmt.Errorf("frontend exited with error: %w", err)
	}

	return nil
}

func buildBackend(goExe, backendDir, backendExe string) error {
	if _, err := os.Stat(backendExe); err == nil {
		return nil
	}

	cmd := exec.Command(goExe, "build", "-o", backendExe, ".")
	cmd.Dir = backendDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func startProcess(exe, dir string, env []string, args ...string) (*exec.Cmd, error) {
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func killProcess(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func waitBackend(url string, timeout time.Duration) error {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil && resp != nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
		}
		time.Sleep(400 * time.Millisecond)
	}
	return errors.New("backend did not become ready in time")
}

func findQtPrefix() (string, error) {
	if v := strings.TrimSpace(os.Getenv("CIALLO_CLAW_QT_PREFIX")); v != "" {
		if _, err := os.Stat(filepath.Join(v, "bin", "qml.exe")); err == nil {
			return v, nil
		}
	}
	for _, candidate := range []string{
		`E:\Qt\6.8.2\mingw_64`,
		`C:\Qt\6.8.2\mingw_64`,
		`D:\Qt\6.8.2\mingw_64`,
	} {
		if _, err := os.Stat(filepath.Join(candidate, "bin", "qml.exe")); err == nil {
			return candidate, nil
		}
	}
	return "", errors.New("Qt prefix not found; set CIALLO_CLAW_QT_PREFIX")
}

func executableDir() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		path = filepath.Clean(path)
	}
	return filepath.Dir(path), nil
}
