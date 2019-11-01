package main

import (
	"fmt"
	"os"
	"time"

	"github.com/alecthomas/kingpin"
)

var (
	appMain       = kingpin.New("worktime", "Shows current worktime of the day and estimates flexi times.")
	argLeaveTime  = appMain.Flag("leave", "Show statistics for a given leave time in format '15:04'").Short('l').String()
	argTargetTime = appMain.Flag("target-time", "Your daily target time like '08:00'").Default("08:00").Short('t').String()
)

var (
	colorLightGray      = "\033[0;37m"
	colorDarkGray       = "\033[1;30m"
	colorRed            = "\033[0;31m"
	colorGreen          = "\033[0;32m"
	colorDarkRed        = "\033[2;31m"
	colorDarkGreen      = "\033[2;32m"
	colorBlue           = "\033[1;34m"
	colorWhite          = "\033[1;37m"
	colorGray           = "\033[2;37m"
	colorEnd            = "\033[0m"
	colorComeEntry      = colorDarkGreen
	colorLeaveEntry     = colorDarkRed
	colorWorkTime       = colorWhite
	colorBreakEntry     = colorGray
	colorBreakInfo      = colorDarkGray
	colorLeaveTime      = colorBlue
	colorFlexiTimePlus  = colorGreen
	colorFlexiTimeMinus = colorRed
)

func main() {
	kingpin.MustParse(appMain.Parse(os.Args[1:]))

	//TODO parameters
	// --go-home --> only show target time reached

	//disableColors()
	if err := process(); err != nil {
		println("%s", err.Error())
		os.Exit(1)
	}
}

func disableColors() {
	colorWorkTime = ""
	colorBreakEntry = ""
	colorBreakInfo = ""
	colorLeaveTime = ""
	colorFlexiTimePlus = ""
	colorFlexiTimeMinus = ""
}

func process() error {
	targetTime := time.Duration(8) * time.Hour
	if len(*argTargetTime) > 0 {
		t, err := time.Parse("15:04", *argTargetTime)
		if err != nil {
			return err
		}
		targetTime = time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute
		//TODO check target time
	}

	dormaHost, err := GetDefaultDormaHost("go-worktime-app")
	if err != nil {
		return err
	}

	user, pass, err := GetCredentials(dormaHost)
	if err != nil {
		return err
	}

	entries, flexiTimeBalance, err := FetchDormaEntries(dormaHost, user, pass)
	if err != nil {
		return err
	}

	if len(entries) > 0 {
		if len(*argLeaveTime) > 0 {
			t, err := time.Parse("15:04", *argLeaveTime)
			if err != nil {
				return err
			}
			now := time.Now()
			leaveTime := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, time.Local)
			//TODO check leaveTime
			entries = append(entries, Entry{Type: EntryTypeLeave, Time: leaveTime})
		}

		for _, entry := range entries {
			if entry.Type == EntryTypeCome {
				println(" %s--> %s%s", colorComeEntry, entry.Time.Format("15:04"), colorEnd)
			} else if entry.Type == EntryTypeLeave {
				println(" %s<-- %s%s", colorLeaveEntry, entry.Time.Format("15:04"), colorEnd)
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

		fmt.Println("-------------------------------------------")
		flexiTime := noSeconds(accountedWorkTime) - targetTime
		println("worktime:            %s%s%s (%s)", colorWorkTime, formatDurationSeconds(accountedWorkTime), colorEnd, formatFlexiTime(flexiTime))
		if noSeconds(accountedBreakTime) != noSeconds(breakTime) {
			println("%sbreak:               %s (taken %s)%s", colorBreakEntry, formatDurationMinutes(accountedBreakTime), formatDurationMinutes(breakTime), colorEnd)
		} else {
			println("%sbreak:               %s%s", colorBreakEntry, formatDurationMinutes(accountedBreakTime), colorEnd)
		}

		newFlexiTimeBalance := flexiTimeBalance + flexiTime
		println("flexi-time balance: %s -> %s", formatFlexiTime(flexiTimeBalance), formatFlexiTime(newFlexiTimeBalance))

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
		t4, err := GetLeaveTime(startTime, breakTime, 10*time.Hour)
		if err != nil {
			return err
		}

		breakTime1 := t1.Sub(startTime) - (6 * time.Hour)
		breakTime2 := t2.Sub(startTime) - targetTime
		breakTime3 := t3.Sub(startTime) - (9 * time.Hour)
		breakTime4 := t4.Sub(startTime) - (10 * time.Hour)

		fmt.Println("-------------------------------------------")
		println("06:00 at %s %s(%s break)%s", t1.Format("15:04"), colorBreakInfo, formatDurationMinutes(breakTime1), colorEnd)
		println("09:00 at %s %s(%s break)%s", t3.Format("15:04"), colorBreakInfo, formatDurationMinutes(breakTime3), colorEnd)
		println("10:00 at %s %s(%s break)%s", t4.Format("15:04"), colorBreakInfo, formatDurationMinutes(breakTime4), colorEnd)
		fmt.Println("-------------------------------------------")
		println("go home (%s) at %s%s%s %s(%s break)%s", formatDurationMinutes(targetTime), colorLeaveTime, t2.Format("15:04"), colorEnd, colorBreakInfo, formatDurationMinutes(breakTime2), colorEnd)
	}

	//TODO print warning "nicht eingestochen" in red

	return nil
}

func noSeconds(t time.Duration) time.Duration {
	return time.Duration(int(t.Minutes())) * time.Minute
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

func formatFlexiTime(d time.Duration) string {
	color := colorFlexiTimePlus
	if d < 0 {
		color = colorFlexiTimeMinus
	}
	return fmt.Sprintf("%s%s%s", color, formatSignedDurationMinutes(d), colorEnd)
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
