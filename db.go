package main

import "time"

type Meeting struct {
	User  string
	Start time.Time
	End   time.Time
}

func UserMeetings(user string, date time.Time) []Meeting {
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

var meetingDB = []Meeting{
	{
		User:  "miki",
		Start: asTime("2026-06-07 08:30"),
		End:   asTime("2026-06-07 09:30"),
	},
	{
		User:  "miki",
		Start: asTime("2026-06-07 13:30"),
		End:   asTime("2026-06-07 14:15"),
	},
	{
		User:  "bill",
		Start: asTime("2026-06-07 09:00"),
		End:   asTime("2026-06-07 09:45"),
	},
	{
		User:  "bill",
		Start: asTime("2026-06-07 13:00"),
		End:   asTime("2026-06-07 14:00"),
	},
}
