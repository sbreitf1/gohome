package main

import (
	"fmt"
	"os"
	"time"

	"github.com/alecthomas/kingpin"
	"github.com/danielb42/goat"
	"github.com/sbreitf1/go-console"
)

var (
	appMain       = kingpin.New("gohome", "Shows current worktime of the day and estimates flexi times.")
	argLeaveTime  = appMain.Flag("leave", "Show statistics for a given leave time in format '15:04'").Short('l').String()
	argTargetTime = appMain.Flag("target-time", "Your daily target time like '08:00'").Default("08:00").Short('t').String()
	argBreakTime  = appMain.Flag("break", "Ignore actual break time and take input like '00:45' instead").Short('b').String()
	argColleagues = appMain.Flag("colleagues", "Show which colleagues are currently here.").Short('c').Bool()
	argPGPKeyName = appMain.Flag("keyname", "Name/ID of GPG key to decrypt hosts-credentials file (deprecated)").Short('k').String()
	argReminder   = appMain.Flag("reminder", "show desktop notification on target time").Short('r').Bool()
)

const (
	colorDarkGray   = "\033[1;30m"
	colorRed        = "\033[0;31m"
	colorGreen      = "\033[0;32m"
	colorDarkRed    = "\033[2;31m"
	colorDarkGreen  = "\033[2;32m"
	colorDarkOrange = "\033[2;33m"
	colorBlue       = "\033[1;34m"
	colorWhite      = "\033[1;37m"
	colorGray       = "\033[2;37m"
	colorEnd        = "\033[0m"
)

var (
	colorComeEntry      = colorDarkGreen
	colorLeaveEntry     = colorDarkRed
	colorTripEntry      = colorDarkOrange
	colorWorkTime       = colorWhite
	colorBreakEntry     = colorGray
	colorBreakInfo      = colorDarkGray
	colorLeaveTime      = colorBlue
	colorFlexiTimePlus  = colorGreen
	colorFlexiTimeMinus = colorRed
)

func main() {
	kingpin.MustParse(appMain.Parse(os.Args[1:]))

	if !console.SupportsColors() {
		disableColors()
	}
	if err := process(); err != nil {
		console.Printlnf("%s", err.Error())
		os.Exit(1)
	}
}

func disableColors() {
	colorComeEntry = ""
	colorLeaveEntry = ""
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
			return fmt.Errorf("Failed to parse target time: %s", err.Error())
		}
		targetTime = time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute
		//TODO check target time
	}

	dormaConfig, err := GetDormaConfig()
	if err != nil {
		return fmt.Errorf("Unable to retrieve Dorma configuration: %s", err.Error())
	}

	if len(*argPGPKeyName) > 0 {
		console.Println("The --keyname (short -k) option is obsolete and will be removed in the future")
	}

	if len(dormaConfig.Pass) == 0 {
		console.Println("Please enter Dorma password (it will not be stored locally):")
		console.Print("> ")
		dormaConfig.Pass, err = console.ReadPassword()
		if err != nil {
			return fmt.Errorf("Unable to retrieve Dorma password: %s", err.Error())
		}
	}

	entries, flexiTimeBalance, colleagues, err := FetchDormaEntries(dormaConfig)
	if err != nil {
		return err
	}

	if *argColleagues {
		for _, c := range colleagues {
			if !c.LoggedIn {
				console.Printlnf("%s%s%s", colorRed, c.Name, colorEnd)
			} else if c.InHomeOffice {
				console.Printlnf("%s%s%s %s%s%s", colorDarkGreen, c.Name, colorEnd, colorGray, "(in home office)", colorEnd)
			} else {
				console.Printlnf("%s%s%s", colorGreen, c.Name, colorEnd)
			}
		}

		return nil
	}

	if len(entries) > 0 {
		if len(*argLeaveTime) > 0 {
			t, err := time.Parse("15:04", *argLeaveTime)
			if err != nil {
				return fmt.Errorf("Failed to parse lave time: %s", err.Error())
			}
			now := time.Now()
			leaveTime := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, time.Local)
			//TODO check leaveTime
			entries = append(entries, Entry{Type: EntryTypeLeave, Time: leaveTime})
		}

		for _, entry := range entries {
			if entry.Type == EntryTypeCome {
				console.Printlnf(" %s--> %s%s", colorComeEntry, entry.Time.Format("15:04"), colorEnd)
			} else if entry.Type == EntryTypeLeave {
				console.Printlnf(" %s<-- %s%s", colorLeaveEntry, entry.Time.Format("15:04"), colorEnd)
			} else if entry.Type == EntryTypeTrip {
				console.Printlnf(" %s<-- %s DG%s", colorTripEntry, entry.Time.Format("15:04"), colorEnd)
			}
		}

		workTime, startTime, breakTime, err := ComputeWorkTime(entries)
		if err != nil {
			return err
		}

		if len(*argBreakTime) > 0 {
			t, err := time.Parse("15:04", *argBreakTime)
			if err != nil {
				return fmt.Errorf("Failed to parse break time: %s", err.Error())
			}

			newBreakTime := time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute
			diff := (breakTime - newBreakTime)
			breakTime = newBreakTime
			workTime += diff
		}

		accountedWorkTime, accountedBreakTime, err := ComputeAccountedWorkTime(workTime, breakTime)
		if err != nil {
			return err
		}

		console.Println("-------------------------------------------")
		flexiTime := noSeconds(accountedWorkTime) - targetTime
		console.Printlnf("worktime:            %s%s%s (%s)", colorWorkTime, formatDurationSeconds(accountedWorkTime), colorEnd, formatFlexiTime(flexiTime))
		if noSeconds(accountedBreakTime) != noSeconds(breakTime) {
			console.Printlnf("%sbreak:               %s (taken %s)%s", colorBreakEntry, formatDurationMinutes(accountedBreakTime), formatDurationMinutes(breakTime), colorEnd)
		} else {
			console.Printlnf("%sbreak:               %s%s", colorBreakEntry, formatDurationMinutes(accountedBreakTime), colorEnd)
		}

		newFlexiTimeBalance := flexiTimeBalance + flexiTime
		console.Printlnf("flexi-time balance: %s -> %s", formatFlexiTime(flexiTimeBalance), formatFlexiTime(newFlexiTimeBalance))

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

		console.Println("-------------------------------------------")
		console.Printlnf("06:00 at %s %s(%s break)%s", t1.Format("15:04"), colorBreakInfo, formatDurationMinutes(breakTime1), colorEnd)
		console.Printlnf("09:00 at %s %s(%s break)%s", t3.Format("15:04"), colorBreakInfo, formatDurationMinutes(breakTime3), colorEnd)
		console.Printlnf("10:00 at %s %s(%s break)%s", t4.Format("15:04"), colorBreakInfo, formatDurationMinutes(breakTime4), colorEnd)
		console.Println("-------------------------------------------")
		console.Printlnf("go home (%s) at %s%s%s %s(%s break)%s", formatDurationMinutes(targetTime), colorLeaveTime, t2.Format("15:04"), colorEnd, colorBreakInfo, formatDurationMinutes(breakTime2), colorEnd)

		if *argReminder {
			goat.ClearQueue("g")
			goat.AddJob("notify-send -i error 'Go home!'", t2, "g")
			goat.AddJob("notify-send -i error '10h-Limit in 15 min! GO HOME!'", t4.Add(-15*time.Minute), "g")
		}
	}

	//TODO print warning "nicht eingestochen" in red

	return nil
}

func noSeconds(t time.Duration) time.Duration {
	return time.Duration(int(t.Minutes())) * time.Minute
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
