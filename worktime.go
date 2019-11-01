package main

import (
	"fmt"
	"time"
)

var (
	// ErrNoEntries is returned when no entries are available for computation.
	ErrNoEntries = fmt.Errorf("no entries")
	// ErrMaxTimeReached is returned when a solution would exceed the maximum working time.
	ErrMaxTimeReached = fmt.Errorf("a maximum working time of 10 hours per day is allowed")
	// ErrOutOfBusinessHours is returned when a solution is outside of the allowed business working hours.
	ErrOutOfBusinessHours = fmt.Errorf("business hours are from 6:30 to 21:00")
)

// Entry describes an entry for coming or leaving to a given time.
type Entry struct {
	Type EntryType
	Time time.Time
}

// EntryType denotes whether an entry is for coming or leaving the company.
type EntryType string

// ComputeWorkTime returns the actual work time, start time and taken break from a set of entries.
func ComputeWorkTime(entries []Entry) (time.Duration, time.Time, time.Duration, error) {
	if len(entries) == 0 {
		return 0, time.Unix(0, 0), 0, ErrNoEntries
	}

	//TODO sort entries by time

	if entries[0].Type != EntryTypeCome {
		return 0, time.Unix(0, 0), 0, fmt.Errorf("did you work all night?")
	}
	if (entries[0].Time.Year() != entries[len(entries)-1].Time.Year()) || (entries[0].Time.Month() != entries[len(entries)-1].Time.Month()) || (entries[0].Time.Day() != entries[len(entries)-1].Time.Day()) {
		return 0, time.Unix(0, 0), 0, fmt.Errorf("list of entries must be for the same day")
	}

	if entries[len(entries)-1].Type != EntryTypeLeave {
		//TODO check entry is for today

		// current in working time slot? end it by virtual leave entry at the current time for live computation
		entries = append(entries, Entry{Type: EntryTypeLeave, Time: time.Now()})
	}

	workTime := time.Duration(0)
	for i := 0; i < (len(entries) - 1); i += 2 {
		if entries[i].Type != EntryTypeCome {
			return 0, time.Unix(0, 0), 0, fmt.Errorf("expected come entry at index %d", i)
		}
		if entries[i+1].Type != EntryTypeLeave {
			return 0, time.Unix(0, 0), 0, fmt.Errorf("expected leave entry at index %d", i+1)
		}

		slotTime := entries[i+1].Time.Sub(entries[i].Time)
		workTime += slotTime
	}

	presenceTime := entries[len(entries)-1].Time.Sub(entries[0].Time)
	breakTime := presenceTime - workTime
	return workTime, entries[0].Time, breakTime, nil
}

// ComputeAccountedWorkTime returns the accounted work and break times according to country policies.
func ComputeAccountedWorkTime(workTime, breakTime time.Duration) (time.Duration, time.Duration, error) {
	// 09:10 - 15:37 -> 06:00 work, 00:27 break
	// 08:08 - 17:38 -> 09:00 work, 00:32 break
	// after 6 hours, the work time only increases when the break time is 30
	// after 9 hours, the work time only increases when the break time is 45

	if workTime > (6 * time.Hour) {
		if breakTime < (30 * time.Minute) {
			if (workTime + breakTime - 6*time.Hour) < (30 * time.Minute) {
				breakTime = workTime + breakTime - 6*time.Hour
				workTime = 6 * time.Hour
			} else {
				workTime = workTime + breakTime - 30*time.Minute
				breakTime = 30 * time.Minute
			}
		}
	}

	if workTime > (9 * time.Hour) {
		if breakTime < (45 * time.Minute) {
			if (workTime + breakTime - 9*time.Hour) < (45 * time.Minute) {
				breakTime = workTime + breakTime - 9*time.Hour
				workTime = 9 * time.Hour
			} else {
				workTime = workTime + breakTime - 45*time.Minute
				breakTime = 45 * time.Minute
			}
		}
	}

	// are the corrected values still above 10h?
	if workTime > (10 * time.Hour) {
		breakTime = workTime + breakTime - 10*time.Hour
		workTime = 10 * time.Hour
	}

	return workTime, breakTime, nil
}

// GetLeaveTime returns the minimal time of day that results in a target accounted work time.
func GetLeaveTime(startTime time.Time, breakTime, targetWorkTime time.Duration) (time.Time, error) {
	//TODO is reachable before 21:00 ?

	if targetWorkTime > (10 * time.Hour) {
		return time.Unix(0, 0), ErrMaxTimeReached
	}

	// dumb way of finding the target time
	for workTime := targetWorkTime; ; workTime += time.Minute {
		accountedWorkTime, accountedBreakTime, err := ComputeAccountedWorkTime(workTime, breakTime)
		if err != nil {
			return time.Unix(0, 0), err
		}

		if accountedWorkTime >= targetWorkTime {
			return startTime.Add(accountedWorkTime).Add(accountedBreakTime), nil
		}
	}
}
