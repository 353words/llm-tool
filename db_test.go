package main

import (
	"testing"
	"time"
)

func TestUserMeetings(t *testing.T) {
	ms := UserMeetings("miki", time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC))
	if len(ms) != 2 {
		t.Fatal(ms)
	}

	ms = UserMeetings("miki", time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC))
	if len(ms) != 0 {
		t.Fatal(ms)
	}
}
