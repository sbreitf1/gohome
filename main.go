package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	if err := process(); err != nil {
		println("%s", err.Error())
		os.Exit(1)
	}
}

func process() error {
	targetTime := 8 * time.Hour

	dormaHost, err := GetDefaultDormaHost("go-worktime-app")
	if err != nil {
		return err
	}

	user, pass, err := GetCredentials(dormaHost)
	if err != nil {
		return err
	}

	entries, err := FetchDormaEntries(dormaHost, user, pass)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.Type == EntryTypeCome {
			fmt.Print(" --> ")
		} else if entry.Type == EntryTypeLeave {
			fmt.Print(" <-- ")
		}
		fmt.Println(entry.Time.Format("15:04"))
	}
	if len(entries) == 0 || entries[len(entries)-1].Type != EntryTypeCome {
		println("EINSTECHEN! LOS!")
		if len(entries) == 0 {
			os.Exit(1)
		}
	}

	workTime, startTime, breakTime, err := ComputeWorkTime(entries)
	if err != nil {
		return err
	}

	accountedWorkTime, accountedBreakTime, err := ComputeAccountedWorkTime(workTime, breakTime)
	if err != nil {
		return err
	}

	//TODO flexitime (read from dorma and show here)

	fmt.Println("-------------------------------------------")
	flexiTime := workTime - targetTime
	println("worktime: %s (%s)", formatDurationSeconds(accountedWorkTime), formatSignedDurationMinutes(flexiTime)) //TODO print sign
	if accountedBreakTime != breakTime {
		println("break:    %s (taken %s)", formatDurationMinutes(accountedBreakTime), formatDurationMinutes(breakTime))
	} else {
		println("break:    %s", formatDurationMinutes(accountedBreakTime))
	}
	//TODO current flexitime

	t1, err := GetLeaveTime(startTime, breakTime, 6*time.Hour)
	if err != nil {
		return err
	}
	t2, err := GetLeaveTime(startTime, breakTime, targetTime)
	if err != nil {
		return err
	}
	t3, err := GetLeaveTime(startTime, breakTime, 9*time.Hour)
	if err != nil {
		return err
	}

	breakTime1 := t1.Sub(startTime) - (6 * time.Hour)
	breakTime2 := t2.Sub(startTime) - targetTime
	breakTime3 := t3.Sub(startTime) - (9 * time.Hour)

	fmt.Println("-------------------------------------------")
	println("06:00 at %s (%s break)", t1.Format("15:04"), formatDurationMinutes(breakTime1))
	println("%s at %s (%s break)", formatDurationMinutes(targetTime), t2.Format("15:04"), formatDurationMinutes(breakTime2))
	println("09:00 at %s (%s break)", t3.Format("15:04"), formatDurationMinutes(breakTime3))

	return nil
}

func println(format string, a ...interface{}) {
	fmt.Println(fmt.Sprintf(format, a...))
}

func formatDurationMinutes(d time.Duration) string {
	minutes := int(d.Minutes())
	hours := minutes / 60
	minutes = minutes - (60 * hours)
	return fmt.Sprintf("%02d:%02d", hours, minutes)
}

func formatSignedDurationMinutes(d time.Duration) string {
	sign := "+"
	if d < 0 {
		d = -d
		sign = "-"
	}
	minutes := int(d.Minutes())
	hours := minutes / 60
	minutes = minutes - (60 * hours)
	return fmt.Sprintf("%s%02d:%02d", sign, hours, minutes)
}

func formatDurationSeconds(d time.Duration) string {
	seconds := int(d.Seconds())
	hours := seconds / (60 * 60)
	seconds = seconds - (60 * 60 * hours)
	minutes := seconds / 60
	seconds = seconds - (60 * minutes)
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}
