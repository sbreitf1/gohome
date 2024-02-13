package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sbreitf1/go-console"
)

type colorsDef struct {
	ComeEntry      string
	LeaveEntry     string
	TripEntry      string
	CacheHint      string
	WorkTime       string
	BreakEntry     string
	BreakInfo      string
	LeaveTime      string
	FlexiTimePlus  string
	FlexiTimeMinus string
}

var (
	patternColor = regexp.MustCompile(`^(\d+);(\d+)$`)
	colors       colorsDef
	colorEnd     = "\033[0m"
)

func initColors() {
	if !console.SupportsColors() {
		verbosePrint("disable color support")
		disableColors()
	} else {
		readColors()
	}
	if *argDumpColors {
		dumpColors()
	}
}

func disableColors() {
	colors.ComeEntry = ""
	colors.LeaveEntry = ""
	colors.TripEntry = ""
	colors.CacheHint = ""
	colors.WorkTime = ""
	colors.BreakEntry = ""
	colors.BreakInfo = ""
	colors.LeaveTime = ""
	colors.FlexiTimePlus = ""
	colors.FlexiTimeMinus = ""
	colorEnd = ""
}

func setDefaultColors() {
	colors.ComeEntry = "\033[2;32m"
	colors.LeaveEntry = "\033[2;31m"
	colors.TripEntry = "\033[2;33m"
	colors.CacheHint = "\033[2;37m"
	colors.WorkTime = "\033[1;37m"
	colors.BreakEntry = "\033[2;37m"
	colors.BreakInfo = "\033[1;30m"
	colors.LeaveTime = "\033[1;34m"
	colors.FlexiTimePlus = "\033[0;32m"
	colors.FlexiTimeMinus = "\033[0;31m"
}

func readColors() {
	setDefaultColors()
	dir := getConfigDir()
	data, err := os.ReadFile(filepath.Join(dir, "colors.json"))
	if err != nil {
		if !os.IsNotExist(err) {
			console.Printlnf("failed to read colors.json: %s", err.Error())
		}
		return
	}
	var newColors colorsDef
	if err := json.Unmarshal(data, &newColors); err != nil {
		console.Printlnf("failed to unmarshal colors config")
		return
	}
	verbosePrint("import colors from colors.json")
	importColors(&colors, newColors)
}

func dumpColors() {
	data, err := json.MarshalIndent(shortenColors(colors), "", "  ")
	if err != nil {
		console.Printlnf("failed to marshal colors to json")
		return
	}
	dir := getConfigDir()
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		console.Printlnf("failed to create config dir: %s", err.Error())
		return
	}
	outputFilePath := filepath.Join(dir, "colors.json")
	if err := os.WriteFile(outputFilePath, data, os.ModePerm); err != nil {
		console.Printlnf("failed to create config dir: %s", err.Error())
		return
	}

	console.Printlnf("wrote colors to %s", outputFilePath)
}

func shortenColors(colors colorsDef) colorsDef {
	return colorsDef{
		ComeEntry:      strings.ReplaceAll(strings.ReplaceAll(colors.ComeEntry, "\033[", ""), "m", ""),
		LeaveEntry:     strings.ReplaceAll(strings.ReplaceAll(colors.LeaveEntry, "\033[", ""), "m", ""),
		TripEntry:      strings.ReplaceAll(strings.ReplaceAll(colors.TripEntry, "\033[", ""), "m", ""),
		CacheHint:      strings.ReplaceAll(strings.ReplaceAll(colors.TripEntry, "\033[", ""), "m", ""),
		WorkTime:       strings.ReplaceAll(strings.ReplaceAll(colors.WorkTime, "\033[", ""), "m", ""),
		BreakEntry:     strings.ReplaceAll(strings.ReplaceAll(colors.BreakEntry, "\033[", ""), "m", ""),
		BreakInfo:      strings.ReplaceAll(strings.ReplaceAll(colors.BreakInfo, "\033[", ""), "m", ""),
		LeaveTime:      strings.ReplaceAll(strings.ReplaceAll(colors.LeaveTime, "\033[", ""), "m", ""),
		FlexiTimePlus:  strings.ReplaceAll(strings.ReplaceAll(colors.FlexiTimePlus, "\033[", ""), "m", ""),
		FlexiTimeMinus: strings.ReplaceAll(strings.ReplaceAll(colors.FlexiTimeMinus, "\033[", ""), "m", ""),
	}
}

func importColors(dst *colorsDef, src colorsDef) {
	importColor(&dst.ComeEntry, src.ComeEntry, "ComeEntry")
	importColor(&dst.LeaveEntry, src.LeaveEntry, "LeaveEntry")
	importColor(&dst.TripEntry, src.TripEntry, "TripEntry")
	importColor(&dst.CacheHint, src.CacheHint, "CacheHint")
	importColor(&dst.WorkTime, src.WorkTime, "WorkTime")
	importColor(&dst.BreakEntry, src.BreakEntry, "BreakEntry")
	importColor(&dst.BreakInfo, src.BreakInfo, "BreakInfo")
	importColor(&dst.LeaveTime, src.LeaveTime, "LeaveTime")
	importColor(&dst.FlexiTimePlus, src.FlexiTimePlus, "FlexiTimePlus")
	importColor(&dst.FlexiTimeMinus, src.FlexiTimeMinus, "FlexiTimeMinus")
}

func importColor(dst *string, src string, fieldName string) {
	m := patternColor.FindStringSubmatch(src)
	if len(m) != 3 {
		console.Printlnf("color for %q invalid", fieldName)
		return
	}
	*dst = fmt.Sprintf("\033[%s;%sm", m[1], m[2])
}
