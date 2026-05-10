package languagepolicy

import "testing"

func TestIsEnglishOnlyText(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{name: "english_greeting", text: "hello there", want: true},
		{name: "english_task_request", text: "Inspect workspace notes and answer.", want: true},
		{name: "english_project_phrase", text: "project alpha rollout", want: true},
		{name: "spanish_word", text: "hola", want: false},
		{name: "french_word", text: "bonjour", want: false},
		{name: "pinyin_phrase", text: "ni hao", want: false},
		{name: "mixed_han", text: "你好 hello", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsEnglishOnlyText(test.text); got != test.want {
				t.Fatalf("IsEnglishOnlyText(%q) = %v, want %v", test.text, got, test.want)
			}
		})
	}
}

func TestPreferredReplyLanguage(t *testing.T) {
	if got := PreferredReplyLanguage("bonjour"); got != ReplyLanguageChinese {
		t.Fatalf("expected non-English ASCII input to keep chinese default, got %q", got)
	}
	if got := PreferredReplyLanguage("hello there"); got != ReplyLanguageEnglish {
		t.Fatalf("expected english input to keep english default, got %q", got)
	}
}
