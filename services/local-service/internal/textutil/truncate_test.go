package textutil

import "testing"

func TestTruncateGraphemesPreservesVisibleCharacters(t *testing.T) {
	t.Parallel()

	if got := TruncateGraphemes("根据当前环境，我具备以下主要功能", 10); got != "根据当前环境，..." {
		t.Fatalf("expected chinese text to stay readable, got %q", got)
	}
	if got := TruncateGraphemes("prefix👨‍👩‍👧‍👦suffix", 10); got != "prefix👨‍👩‍👧‍👦..." {
		t.Fatalf("expected family emoji cluster to stay intact, got %q", got)
	}
	if got := TruncateGraphemes("Cafe\u0301 society", 7); got != "Cafe\u0301..." {
		t.Fatalf("expected combining-mark grapheme to stay intact, got %q", got)
	}
	if got := TruncateGraphemes("short", 10); got != "short" {
		t.Fatalf("expected shorter text to stay unchanged, got %q", got)
	}
	if got := TruncateGraphemes("emoji🙂test", 3); got != "emo" {
		t.Fatalf("expected tiny budgets to skip ellipsis and keep boundaries, got %q", got)
	}
}
