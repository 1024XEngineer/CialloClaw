package clipboard

import (
	"context"
	"os/exec"
	"strings"
	"sync"
	"time"

	"cialloclaw/internal/protocol"
	"cialloclaw/internal/runtime/eventbus"
)

type Source struct {
	Bus       eventbus.Bus
	SessionID func() string
	Interval  time.Duration
	mu        sync.Mutex
	lastValue string
	stopCh    chan struct{}
	stopped   bool
}

func New(bus eventbus.Bus, sessionID func() string, interval time.Duration) *Source {
	return &Source{
		Bus:       bus,
		SessionID: sessionID,
		Interval:  interval,
		stopCh:    make(chan struct{}),
	}
}

func (s *Source) Name() string {
	return "perception.clipboard"
}

func (s *Source) Start(ctx context.Context) error {
	go s.loop(ctx)
	return nil
}

func (s *Source) Stop(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return nil
	}
	s.stopped = true
	close(s.stopCh)
	return nil
}

func (s *Source) loop(ctx context.Context) {
	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.poll(ctx)
		}
	}
}

func (s *Source) poll(ctx context.Context) {
	sessionID := s.SessionID()
	if sessionID == "" {
		return
	}
	text, err := readClipboard()
	if err != nil || text == "" {
		return
	}
	s.mu.Lock()
	if text == s.lastValue {
		s.mu.Unlock()
		return
	}
	s.lastValue = text
	s.mu.Unlock()

	payload := protocol.ClipboardChangedPayload{
		Text:       text,
		Kind:       detectKind(text),
		IsURL:      isURL(text),
		IsVideoURL: isVideoURL(text),
	}
	event := protocol.NewEvent(protocol.EventTypeClipboardChanged, "perception.clipboard", sessionID, protocol.PriorityNormal, payload)
	_ = s.Bus.Publish(ctx, event)
}

func readClipboard() (string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "[Console]::OutputEncoding=[System.Text.Encoding]::UTF8; Get-Clipboard -Raw")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func detectKind(text string) string {
	switch {
	case isVideoURL(text):
		return "video_url"
	case isURL(text):
		return "url"
	default:
		return "text"
	}
}

func isURL(text string) bool {
	text = strings.TrimSpace(text)
	return strings.HasPrefix(text, "http://") || strings.HasPrefix(text, "https://")
}

func isVideoURL(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if !isURL(lower) {
		return false
	}
	videoHosts := []string{"youtube.com", "youtu.be", "bilibili.com", "vimeo.com", "tiktok.com"}
	for _, host := range videoHosts {
		if strings.Contains(lower, host) {
			return true
		}
	}
	return false
}
