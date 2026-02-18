package main

import (
	"testing"
	"time"
)

func TestQueryMeetings(t *testing.T) {
	ms := QueryMeetings("miki", time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC))
	if len(ms) != 2 {
		t.Fatal(ms)
	}

	ms = QueryMeetings("miki", time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC))
	if len(ms) != 0 {
		t.Fatal(ms)
	}
}
