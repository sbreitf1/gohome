package stdio

import (
	"bufio"
	"fmt"
	"os"
	"syscall"

	"golang.org/x/term"
)

var (
	Verbose bool
)

func Debug(msg string, args ...interface{}) {
	if Verbose {
		fmt.Println("[DEBUG]", fmt.Sprintf(msg, args...))
	}
}

func Info(msg string, args ...interface{}) {
	fmt.Println("[INFO]", fmt.Sprintf(msg, args...))
}

func Warn(msg string, args ...interface{}) {
	fmt.Println("[WARN]", fmt.Sprintf(msg, args...))
}

func Error(msg string, args ...interface{}) {
	fmt.Println("[ERR]", fmt.Sprintf(msg, args...))
}

func Print(msg string, args ...interface{}) {
	fmt.Print(fmt.Sprintf(msg, args...))
}

func Println(msg string, args ...interface{}) {
	fmt.Println(fmt.Sprintf(msg, args...))
}

func ReadLineWithPrompt(prompt string) (string, error) {
	fmt.Print(prompt)
	return ReadLine()
}

func ReadLine() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	return reader.ReadString('\n')
}

func ReadPasswordWithPrompt(prompt string) (string, error) {
	fmt.Print(prompt)
	return ReadPassword()
}

func ReadPassword() (string, error) {
	pass, err := term.ReadPassword(syscall.Stdin)
	if err != nil {
		return "", err
	}
	return string(pass), nil
}

func SupportsColors() bool {
	//TODO implement SupportsColors
	return true
}
