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
	appMain             = kingpin.New("gohome", "Shows current worktime of the day and estimates flexi times.")
	argLeaveTime        = appMain.Flag("leave", "Show statistics for a given leave time in format '15:04'").Short('l').String()
	argTargetTime       = appMain.Flag("target-time", "Your daily target time like '08:00'").Default("08:00").Short('t').String()
	argBreakTime        = appMain.Flag("break", "Ignore actual break time and take input like '00:45' instead").Short('b').String()
	argReminder         = appMain.Flag("reminder", "Show desktop notification on target time").Short('r').Bool()
	argVerbose          = appMain.Flag("verbose", "Print every single step").Short('v').Bool()
	argForceReload      = appMain.Flag("force-reload", "Do not use existing cache and force refresh of entries").Short('f').Bool()
	argCacheTimeSeconds = appMain.Flag("cache-time", "Max cache age in seconds").Default("600").Int()
	argDumpColors       = appMain.Flag("dump-colors", fmt.Sprintf("Populates %s/colors.json with the current colors", getConfigDir())).Bool()
	currentState        EntryType
)

func verbosePrint(format string, a ...interface{}) {
	if *argVerbose {
		console.Printlnf("[DEBUG] "+format, a...)
	}
}

func main() {
	kingpin.MustParse(appMain.Parse(os.Args[1:]))

	initColors()

	if err := process(); err != nil {
		console.Printlnf("%s", err.Error())
		os.Exit(1)
	}

	if currentState != EntryTypeCome {
		os.Exit(2)
	}
}

func process() error {
	targetTime := time.Duration(8) * time.Hour
	if len(*argTargetTime) > 0 {
		t, err := time.Parse("15:04", *argTargetTime)
		if err != nil {
			return fmt.Errorf("failed to parse target time: %s", err.Error())
		}
		targetTime = time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute
		//TODO check target time
	}

	var entries []Entry
	var flexiTimeBalance time.Duration
	var cacheTime time.Time
	var cacheOK bool
	if !*argForceReload {
		verbosePrint("read cache")
		var err error
		entries, flexiTimeBalance, cacheTime, cacheOK, err = ReadCache()
		if err != nil {
			verbosePrint("read cache failed: %s", err.Error())
		} else if cacheOK {
			if len(entries) == 0 {
				verbosePrint("no entries in cache, force update")
				cacheOK = false
			} else {
				if entries[len(entries)-1].Type != EntryTypeCome {
					verbosePrint("latest entry in cache is %q, force update", entries[len(entries)-1].Type)
					cacheOK = false
				} else {
					verbosePrint("cache is valid")
				}
			}
		}
	}
	if !cacheOK {
		matrixConfig, err := GetMatrixConfig()
		if err != nil {
			return fmt.Errorf("unable to retrieve Matrix configuration: %s", err.Error())
		}

		if len(matrixConfig.Pass) == 0 {
			console.Println("Please enter Matrix password (it will not be stored locally):")
			console.Print("> ")
			matrixConfig.Pass, err = console.ReadPassword()
			if err != nil {
				return fmt.Errorf("unable to retrieve Matrix password: %s", err.Error())
			}
		}

		verbosePrint("fetch matrix entries")
		entries, flexiTimeBalance, err = FetchMatrixEntries(matrixConfig)
		if err != nil {
			return err
		}

		if err := WriteCache(entries, flexiTimeBalance); err != nil {
			verbosePrint("write cache failed: %s", err.Error())
		} else {
			verbosePrint("cache written")
		}
	}

	verbosePrint("entry count: %d", len(entries))
	if len(entries) > 0 {
		if len(*argLeaveTime) > 0 {
			t, err := time.Parse("15:04", *argLeaveTime)
			if err != nil {
				return fmt.Errorf("failed to parse leave time: %s", err.Error())
			}
			now := time.Now()
			leaveTime := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, time.Local)
			//TODO check leaveTime
			entries = append(entries, Entry{Type: EntryTypeLeave, Time: leaveTime})
		}

		for _, entry := range entries {
			if entry.Type == EntryTypeCome {
				console.Printlnf(" %s--> %s%s", colors.ComeEntry, entry.Time.Format("15:04"), colorEnd)
			} else if entry.Type == EntryTypeLeave {
				console.Printlnf(" %s<-- %s%s", colors.LeaveEntry, entry.Time.Format("15:04"), colorEnd)
			} else if entry.Type == EntryTypeTrip {
				console.Printlnf(" %s<-- %s DG%s", colors.TripEntry, entry.Time.Format("15:04"), colorEnd)
			}

			currentState = entry.Type
		}

		workTime, startTime, breakTime, err := ComputeWorkTime(entries)
		if err != nil {
			return err
		}

		if len(*argBreakTime) > 0 {
			t, err := time.Parse("15:04", *argBreakTime)
			if err != nil {
				return fmt.Errorf("failed to parse break time: %s", err.Error())
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

		console.Println("-----------------------------------------------------")
		if cacheOK {
			console.Printlnf("time now:            %s (cache from %s)", time.Now().Format("15:04"), cacheTime.Format("15:04:05"))
		} else {
			console.Printlnf("time now:            %s", time.Now().Format("15:04"))
		}
		flexiTime := noSeconds(accountedWorkTime) - targetTime
		console.Printlnf("worktime:            %s%s%s (%s)", colors.WorkTime, formatDurationSeconds(accountedWorkTime), colorEnd, formatFlexiTime(flexiTime))
		if noSeconds(accountedBreakTime) != noSeconds(breakTime) {
			console.Printlnf("%sbreak:               %s (taken %s)%s", colors.BreakEntry, formatDurationMinutes(accountedBreakTime), formatDurationMinutes(breakTime), colorEnd)
		} else {
			console.Printlnf("%sbreak:               %s%s", colors.BreakEntry, formatDurationMinutes(accountedBreakTime), colorEnd)
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

		console.Println("-----------------------------------------------------")
		console.Printlnf("06:00 at %s %s(%s break)%s", t1.Format("15:04"), colors.BreakInfo, formatDurationMinutes(breakTime1), colorEnd)
		console.Printlnf("09:00 at %s %s(%s break)%s", t3.Format("15:04"), colors.BreakInfo, formatDurationMinutes(breakTime3), colorEnd)
		console.Printlnf("10:00 at %s %s(%s break)%s", t4.Format("15:04"), colors.BreakInfo, formatDurationMinutes(breakTime4), colorEnd)
		console.Println("-----------------------------------------------------")
		console.Printlnf("go home (%s) at %s%s%s %s(%s break)%s", formatDurationMinutes(targetTime), colors.LeaveTime, t2.Format("15:04"), colorEnd, colors.BreakInfo, formatDurationMinutes(breakTime2), colorEnd)

		if *argReminder {
			if err := goat.ClearQueue("g"); err != nil {
				return fmt.Errorf("clear job queue: %s", err.Error())
			}
			if _, err := goat.AddJob("notify-send -i error 'Go home!'", t2, "g"); err != nil {
				return fmt.Errorf("add gohome job: %s", err.Error())
			}
			if _, err := goat.AddJob("notify-send -i error '10h-Limit in 15 min! GO HOME!'", t4.Add(-15*time.Minute), "g"); err != nil {
				return fmt.Errorf("add 10h warning job: %s", err.Error())
			}
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
	color := colors.FlexiTimePlus
	if d < 0 {
		color = colors.FlexiTimeMinus
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
