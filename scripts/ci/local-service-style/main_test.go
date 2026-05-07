package main

import "testing"

func TestFindCommentViolationsRejectsAddedChineseComments(t *testing.T) {
	diff := `diff --git a/services/local-service/internal/demo/demo.go b/services/local-service/internal/demo/demo.go
index 1111111..2222222 100644
--- a/services/local-service/internal/demo/demo.go
+++ b/services/local-service/internal/demo/demo.go
@@ -1,0 +2,4 @@
+package demo
+// Load loads 配置.
+func Load() {}
+var _ = 1 /* 中文 block */
`

	violations := findCommentViolations(diff)
	if len(violations) != 2 {
		t.Fatalf("expected two violations, got %d: %#v", len(violations), violations)
	}
	if violations[0].line != 3 {
		t.Fatalf("expected first violation on line 3, got %d", violations[0].line)
	}
	if violations[1].line != 5 {
		t.Fatalf("expected second violation on line 5, got %d", violations[1].line)
	}
}

func TestFindCommentViolationsIgnoresChineseStringLiterals(t *testing.T) {
	diff := `diff --git a/services/local-service/internal/demo/demo.go b/services/local-service/internal/demo/demo.go
index 1111111..2222222 100644
--- a/services/local-service/internal/demo/demo.go
+++ b/services/local-service/internal/demo/demo.go
@@ -1,0 +2,3 @@
+package demo
+const message = "中文 // not a comment"
+const raw = ` + "`" + `中文 /* not a comment */` + "`" + `
`

	violations := findCommentViolations(diff)
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %#v", violations)
	}
}

func TestFindCommentViolationsTracksAddedBlockComments(t *testing.T) {
	diff := `diff --git a/services/local-service/internal/demo/demo.go b/services/local-service/internal/demo/demo.go
index 1111111..2222222 100644
--- a/services/local-service/internal/demo/demo.go
+++ b/services/local-service/internal/demo/demo.go
@@ -1,0 +2,5 @@
+package demo
+/*
+English line.
+中文 line.
+*/
`

	violations := findCommentViolations(diff)
	if len(violations) != 1 {
		t.Fatalf("expected one violation, got %d: %#v", len(violations), violations)
	}
	if violations[0].line != 5 {
		t.Fatalf("expected violation on line 5, got %d", violations[0].line)
	}
}
