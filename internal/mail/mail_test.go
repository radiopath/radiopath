package mail

import (
	"strings"
	"testing"
	"time"
)

func TestMessage(t *testing.T) {
	now := time.Date(2026, 9, 6, 20, 0, 0, 0, time.FixedZone("CEST", 7200))
	got := string(message("noreply@radiopath.example", "op@hb9hil.example", "Hello", "line one\nline two\n", now))
	head, body, ok := strings.Cut(got, "\r\n\r\n")
	if !ok {
		t.Fatal("no header/body separator")
	}
	for _, want := range []string{
		"From: noreply@radiopath.example\r\n",
		"To: op@hb9hil.example\r\n",
		"Subject: Hello\r\n",
		"Date: Sun, 06 Sep 2026 20:00:00 +0200\r\n",
		"@radiopath.example>\r\n",
		"Content-Type: text/plain; charset=utf-8\r\n",
	} {
		if !strings.Contains(head+"\r\n", want) {
			t.Errorf("header missing %q in\n%s", want, head)
		}
	}
	if body != "line one\r\nline two\r\n" {
		t.Errorf("body %q", body)
	}
}
