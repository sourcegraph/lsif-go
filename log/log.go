package log

import (
	"log"
	"os"
)

func init() {
	log.SetFlags(0)
	log.SetPrefix("")
	log.SetOutput(os.Stdout)
}

// Level determines the level of verbose for logging messages.
type Level int

// Logging levels can be used to define verboseness.
const (
	Debug Level = iota
	Info
	None Level = 99
)

var level = None

// SetLevel sets the logging level.
func SetLevel(l Level) {
	level = l
}

// Debugf prints logging messages in Debug level.
// Arguments are handled in the manner of fmt.Printf.
func Debugf(format string, v ...interface{}) {
	if level > Debug {
		return
	}

	log.Printf(format, v...)
}

// Debugf prints logging messages in Debug level.
// Arguments are handled in the manner of fmt.Println.
func Debugln(v ...interface{}) {
	if level > Debug {
		return
	}

	log.Println(v...)
}

// Infof prints logging messages in Info level.
// Arguments are handled in the manner of fmt.Printf.
func Infof(format string, v ...interface{}) {
	if level > Info {
		return
	}

	log.Printf(format, v...)
}

// Infoln prints logging messages in Info level.
// Arguments are handled in the manner of fmt.Println.
func Infoln(v ...interface{}) {
	if level > Info {
		return
	}

	log.Println(v...)
}
