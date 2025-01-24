package main

import (
	"fmt"
	"os"
	"time"

	"github.com/sbreitf1/gohome/internal/pkg/stdio"

	"github.com/alecthomas/kong"
	"github.com/danielb42/goat"
)

var (
	cli struct {
		Verbose bool `name:"verbose" short:"v" help:"more verbose printing"`
		Debug   bool `name:"debug" help:"maximum debug output including scraped files"`

		Show struct {
			TargetTime       string `name:"target-time" short:"t" default:"08:00" help:"assume target time in format '15:04'"`
			LeaveTime        string `name:"leave-time" short:"l" default:"" help:"simulate a given leave time in format '15:04'"`
			BreakTime        string `name:"break-time" short:"b" default:"" help:"simulate a given break time in format '15:04'"`
			ForceReload      bool   `name:"force-reload" short:"f" help:"ignore local cache and force refresh of entries"`
			CacheTimeSeconds int    `name:"cache-time" default:"600" help:"max cache age in seconds"`
			SetReminder      bool   `name:"set-reminder" short:"r" help:"sets a reminder via linux at command"`

			SaveConfig bool `name:"save-config" help:"DEPRECATED - write changes from command line parameters to user config"`
		} `cmd:"show" default:"withargs" help:"Show today's stats"`

		DumpColors struct {
		} `cmd:"dump-colors" help:"Populates colors.json in the application config directory"`
	}

	currentState EntryType
)

func main() {
	ctx := kong.Parse(&cli)
	if err := execCmd(ctx.Command()); err != nil {
		fmt.Println("ERR:", err)
		os.Exit(1)
	}
}

func execCmd(cmd string) error {
	if cli.Debug {
		cli.Verbose = true
		matrixDebugPrint = true
		matrixOutputFiles = true
		matrixOutputFileDir = "."
	}
	stdio.Verbose = cli.Verbose

	initColors()

	switch cmd {
	case "show":
		return cmdShow()

	case "dump-colors":
		return dumpColors()

	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func cmdShow() error {
	if err := process(); err != nil {
		return err
	}

	if currentState != EntryTypeCome {
		stdio.Warn("clock is not ticking at the moment!")
		os.Exit(2)
	}
	return nil
}

func process() error {
	var usrConfIsOK bool
	usrConf, err := ReadUserConfig()
	if err != nil {
		stdio.Warn("read user config failed: %s", err.Error())
	} else {
		usrConfIsOK = true
	}

	if len(cli.Show.TargetTime) == 0 && len(usrConf.TargetTimeStr) > 0 {
		cli.Show.TargetTime = usrConf.TargetTimeStr
	}

	targetTime := time.Duration(8) * time.Hour
	if len(cli.Show.TargetTime) > 0 {
		t, err := time.Parse("15:04", cli.Show.TargetTime)
		if err != nil {
			return fmt.Errorf("failed to parse target time: %s", err.Error())
		}
		targetTime = time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute
		//TODO check target time

		usrConf.TargetTimeStr = fmt.Sprintf("%02d:%02d", t.Hour(), t.Minute())
	}

	if cli.Show.SaveConfig {
		if usrConfIsOK {
			stdio.Debug("persist user config")
			if err := WriteUserConfig(usrConf); err != nil {
				stdio.Warn("write user config failed: %s", err.Error())
			}
		} else {
			stdio.Debug("skip export of corrupt user config")
		}
	}

	stdio.Debug("target time is %v", targetTime)

	var entries []Entry
	var flexiTimeBalance time.Duration
	var cacheTime time.Time
	var cacheOK bool
	if !cli.Show.ForceReload {
		stdio.Debug("read cache")
		var err error
		entries, flexiTimeBalance, cacheTime, cacheOK, err = ReadCache()
		if err != nil {
			stdio.Warn("read cache failed: %s", err.Error())
		} else if cacheOK {
			if len(entries) == 0 {
				stdio.Debug("no entries in cache, force update")
				cacheOK = false
			} else {
				if entries[len(entries)-1].Type != EntryTypeCome {
					stdio.Debug("latest entry in cache is %q, force update", entries[len(entries)-1].Type)
					cacheOK = false
				} else {
					stdio.Debug("cache is valid")
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
			stdio.Println("Please enter Matrix password (it will not be stored locally):")
			matrixConfig.Pass, err = stdio.ReadPasswordWithPrompt("> ")
			if err != nil {
				return fmt.Errorf("unable to retrieve Matrix password: %s", err.Error())
			}
		}

		stdio.Debug("fetch matrix entries")
		entries, flexiTimeBalance, err = FetchMatrixEntries(matrixConfig)
		if err != nil {
			return err
		}

		if err := WriteCache(entries, flexiTimeBalance); err != nil {
			stdio.Warn("write cache failed: %s", err.Error())
		} else {
			stdio.Debug("cache written")
		}
	}

	stdio.Debug("entry count: %d", len(entries))
	if len(entries) > 0 {
		currentState = entries[len(entries)-1].Type

		if len(cli.Show.LeaveTime) > 0 {
			t, err := time.Parse("15:04", cli.Show.LeaveTime)
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
				stdio.Println(" %s--> %s%s", colors.ComeEntry, entry.Time.Format("15:04"), colorEnd)
			} else if entry.Type == EntryTypeLeave {
				stdio.Println(" %s<-- %s%s", colors.LeaveEntry, entry.Time.Format("15:04"), colorEnd)
			} else if entry.Type == EntryTypeTrip {
				stdio.Println(" %s<-- %s DG%s", colors.TripEntry, entry.Time.Format("15:04"), colorEnd)
			}
		}

		workTime, startTime, breakTime, err := ComputeWorkTime(entries)
		if err != nil {
			return err
		}

		if len(cli.Show.BreakTime) > 0 {
			t, err := time.Parse("15:04", cli.Show.BreakTime)
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

		stdio.Println("-----------------------------------------------------")
		if cacheOK {
			stdio.Println("time now:            %s %s(cache from %s)%s", time.Now().Format("15:04"), colors.CacheHint, cacheTime.Format("15:04:05"), colorEnd)
		} else {
			stdio.Println("time now:            %s", time.Now().Format("15:04"))
		}
		flexiTime := noSeconds(accountedWorkTime) - targetTime
		stdio.Println("worktime:            %s%s%s (%s)", colors.WorkTime, formatDurationSeconds(accountedWorkTime), colorEnd, formatFlexiTime(flexiTime))
		if noSeconds(accountedBreakTime) != noSeconds(breakTime) {
			stdio.Println("%sbreak:               %s (taken %s)%s", colors.BreakEntry, formatDurationMinutes(accountedBreakTime), formatDurationMinutes(breakTime), colorEnd)
		} else {
			stdio.Println("%sbreak:               %s%s", colors.BreakEntry, formatDurationMinutes(accountedBreakTime), colorEnd)
		}

		newFlexiTimeBalance := flexiTimeBalance + flexiTime
		stdio.Println("flexi-time balance: %s -> %s", formatFlexiTime(flexiTimeBalance), formatFlexiTime(newFlexiTimeBalance))

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

		stdio.Println("-----------------------------------------------------")
		stdio.Println("06:00 at %s %s(%s break)%s", t1.Format("15:04"), colors.BreakInfo, formatDurationMinutes(breakTime1), colorEnd)
		stdio.Println("09:00 at %s %s(%s break)%s", t3.Format("15:04"), colors.BreakInfo, formatDurationMinutes(breakTime3), colorEnd)
		stdio.Println("10:00 at %s %s(%s break)%s", t4.Format("15:04"), colors.BreakInfo, formatDurationMinutes(breakTime4), colorEnd)
		stdio.Println("-----------------------------------------------------")
		stdio.Println("go home (%s) at %s%s%s %s(%s break)%s", formatDurationMinutes(targetTime), colors.LeaveTime, t2.Format("15:04"), colorEnd, colors.BreakInfo, formatDurationMinutes(breakTime2), colorEnd)

		if cli.Show.SetReminder {
			if err := goat.ClearQueue("g"); err != nil {
				return fmt.Errorf("clear job queue: %s", err.Error())
			}
			if _, err := goat.AddJob("notify-send -i error 'Go home!'", t2, "g"); err != nil {
				return fmt.Errorf("add gohome job: %s", err.Error())
			}
			t5 := t4.Add(-15 * time.Minute)
			if _, err := goat.AddJob("notify-send -i error '10h-Limit in 15 min! GO HOME!'", t5, "g"); err != nil {
				return fmt.Errorf("add 10h warning job: %s", err.Error())
			}
			stdio.Println("-----------------------------------------------------")
			stdio.Println("-> reminders have been set to %d:%d and %d:%d", t2.Hour(), t2.Minute(), t5.Hour(), t5.Minute())
		}
	}

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
