package main

import (
	_ "embed"
	"encoding/csv"
	"errors"
	"io"
	"strings"
	"time"
)

type Meeting struct {
	User  string
	Start time.Time
	End   time.Time
}

func QueryMeetings(user string, date time.Time) []Meeting {
	var meetings []Meeting

	for _, m := range meetingDB {
		if m.User == user && sameDate(m.Start, date) {
			meetings = append(meetings, m)
		}
	}

	return meetings
}

func sameDate(t1, t2 time.Time) bool {
	return t1.Year() == t2.Year() && t1.Month() == t2.Month() && t1.Day() == t2.Day()
}

func asTime(s string) time.Time {
	t, err := time.Parse("2006-01-02 15:04", s)
	if err != nil {
		panic(err)
	}

	return t
}

var meetingDB []Meeting

func init() {
	r := csv.NewReader(strings.NewReader(csvData))
	for {
		row, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			panic(err)
		}

		m := Meeting{
			User: row[0],
		}

		date, err := time.Parse("2006-01-02", row[1])
		if err != nil {
			panic(err)
		}

		start, err := time.Parse("15:04", row[2])
		if err != nil {
			panic(err)
		}
		m.Start = time.Date(date.Year(), date.Month(), date.Day(), start.Hour(), start.Minute(), 0, 0, time.UTC)

		end, err := time.Parse("15:04", row[3])
		if err != nil {
			panic(err)
		}
		m.End = time.Date(date.Year(), date.Month(), date.Day(), end.Hour(), end.Minute(), 0, 0, time.UTC)

		meetingDB = append(meetingDB, m)
	}
}

//go:embed meetings.csv
var csvData string
