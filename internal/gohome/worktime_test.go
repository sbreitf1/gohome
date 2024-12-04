package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type accTimeCase struct {
	WorkTime, BreakTime       time.Duration
	AccWorkTime, AccBreakTime time.Duration
	ExpectError               bool
}

func TestComputeAccountedWorkTime(t *testing.T) {
	testCases := []accTimeCase{
		{WorkTime: dur(5, 0), BreakTime: dur(0, 0), AccWorkTime: dur(5, 0), AccBreakTime: dur(0, 0)},
		{WorkTime: dur(5, 0), BreakTime: dur(0, 30), AccWorkTime: dur(5, 0), AccBreakTime: dur(0, 30)},
		{WorkTime: dur(5, 0), BreakTime: dur(1, 47), AccWorkTime: dur(5, 0), AccBreakTime: dur(1, 47)},
		{WorkTime: dur(6, 0), BreakTime: dur(0, 15), AccWorkTime: dur(6, 0), AccBreakTime: dur(0, 15)},
		{WorkTime: dur(6, 1), BreakTime: dur(0, 30), AccWorkTime: dur(6, 1), AccBreakTime: dur(0, 30)},
		{WorkTime: dur(6, 1), BreakTime: dur(0, 0), AccWorkTime: dur(6, 0), AccBreakTime: dur(0, 1)},
		{WorkTime: dur(6, 1), BreakTime: dur(0, 29), AccWorkTime: dur(6, 0), AccBreakTime: dur(0, 30)},
		{WorkTime: dur(6, 39), BreakTime: dur(0, 15), AccWorkTime: dur(6, 24), AccBreakTime: dur(0, 30)},
		{WorkTime: dur(6, 17), BreakTime: dur(0, 38), AccWorkTime: dur(6, 17), AccBreakTime: dur(0, 38)},
		{WorkTime: dur(8, 44), BreakTime: dur(0, 0), AccWorkTime: dur(8, 14), AccBreakTime: dur(0, 30)},
		{WorkTime: dur(8, 27), BreakTime: dur(0, 13), AccWorkTime: dur(8, 10), AccBreakTime: dur(0, 30)},
		{WorkTime: dur(8, 59), BreakTime: dur(0, 0), AccWorkTime: dur(8, 29), AccBreakTime: dur(0, 30)},
		{WorkTime: dur(8, 39), BreakTime: dur(0, 36), AccWorkTime: dur(8, 39), AccBreakTime: dur(0, 36)},
		{WorkTime: dur(8, 51), BreakTime: dur(1, 07), AccWorkTime: dur(8, 51), AccBreakTime: dur(1, 7)},
		{WorkTime: dur(9, 0), BreakTime: dur(0, 0), AccWorkTime: dur(8, 30), AccBreakTime: dur(0, 30)},
		{WorkTime: dur(9, 1), BreakTime: dur(0, 0), AccWorkTime: dur(8, 31), AccBreakTime: dur(0, 30)},
		{WorkTime: dur(9, 45), BreakTime: dur(0, 0), AccWorkTime: dur(9, 0), AccBreakTime: dur(0, 45)},
		{WorkTime: dur(9, 50), BreakTime: dur(0, 0), AccWorkTime: dur(9, 5), AccBreakTime: dur(0, 45)},
		{WorkTime: dur(9, 45), BreakTime: dur(0, 13), AccWorkTime: dur(9, 13), AccBreakTime: dur(0, 45)},
		{WorkTime: dur(9, 20), BreakTime: dur(0, 13), AccWorkTime: dur(9, 0), AccBreakTime: dur(0, 33)},
		{WorkTime: dur(9, 58), BreakTime: dur(0, 46), AccWorkTime: dur(9, 58), AccBreakTime: dur(0, 46)},
		{WorkTime: dur(9, 1), BreakTime: dur(0, 38), AccWorkTime: dur(9, 0), AccBreakTime: dur(0, 39)},
		{WorkTime: dur(9, 9), BreakTime: dur(0, 41), AccWorkTime: dur(9, 5), AccBreakTime: dur(0, 45)},
		{WorkTime: dur(10, 5), BreakTime: dur(0, 40), AccWorkTime: dur(10, 0), AccBreakTime: dur(0, 45)},
		{WorkTime: dur(10, 17), BreakTime: dur(0, 48), AccWorkTime: dur(10, 0), AccBreakTime: dur(1, 5)},
	}

	for _, c := range testCases {
		t.Run(fmt.Sprintf("Test %s, %s", c.WorkTime, c.BreakTime), func(t *testing.T) {
			accWorkTime, accBreakTime, err := ComputeAccountedWorkTime(c.WorkTime, c.BreakTime)
			if c.ExpectError {
				assert.Error(t, err)
			} else {
				assert.Equal(t, c.AccWorkTime, accWorkTime)
				assert.Equal(t, c.AccBreakTime, accBreakTime)
			}
		})
	}
}

type leaveCase struct {
	StartTime                 time.Time
	BreakTime, TargetWorkTime time.Duration
	LeaveTime                 time.Time
	ExpectError               bool
}

func TestGetLeaveTime(t *testing.T) {
	testCases := []leaveCase{
		{StartTime: tim(8, 00), BreakTime: dur(00, 00), TargetWorkTime: dur(6, 00), LeaveTime: tim(14, 00)},
		{StartTime: tim(8, 00), BreakTime: dur(00, 15), TargetWorkTime: dur(6, 00), LeaveTime: tim(14, 15)},
		{StartTime: tim(8, 00), BreakTime: dur(00, 15), TargetWorkTime: dur(6, 00), LeaveTime: tim(14, 15)},
		{StartTime: tim(8, 00), BreakTime: dur(00, 00), TargetWorkTime: dur(8, 00), LeaveTime: tim(16, 30)},
		{StartTime: tim(8, 00), BreakTime: dur(00, 39), TargetWorkTime: dur(8, 00), LeaveTime: tim(16, 39)},
		{StartTime: tim(8, 00), BreakTime: dur(00, 15), TargetWorkTime: dur(8, 00), LeaveTime: tim(16, 30)},
		{StartTime: tim(8, 00), BreakTime: dur(00, 15), TargetWorkTime: dur(9, 00), LeaveTime: tim(17, 30)},
		{StartTime: tim(8, 00), BreakTime: dur(00, 15), TargetWorkTime: dur(9, 30), LeaveTime: tim(18, 15)},
		{StartTime: tim(8, 00), BreakTime: dur(00, 50), TargetWorkTime: dur(9, 30), LeaveTime: tim(18, 20)},
		{StartTime: tim(8, 00), BreakTime: dur(01, 00), TargetWorkTime: dur(10, 00), LeaveTime: tim(19, 00)},
		{TargetWorkTime: dur(10, 15), ExpectError: true},
	}

	for _, c := range testCases {
		t.Run(fmt.Sprintf("Test %s, %s, %s", c.LeaveTime, c.BreakTime, c.TargetWorkTime), func(t *testing.T) {
			leaveTime, err := GetLeaveTime(c.StartTime, c.BreakTime, c.TargetWorkTime)
			if c.ExpectError {
				assert.Error(t, err)
			} else {
				assert.Equal(t, c.LeaveTime, leaveTime)
			}
		})
	}
}

func dur(hours, minutes int) time.Duration {
	return time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute
}

func tim(hours, minutes int) time.Time {
	return time.Date(2019, time.November, 1, hours, minutes, 0, 0, time.UTC)
}
