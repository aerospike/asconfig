package log

import (
	"github.com/sirupsen/logrus"
)

// log levels are the same as logrus.
func GetLogLevels() (levels []string) {
	levels = make([]string, len(logrus.AllLevels))

	for i, lvl := range logrus.AllLevels {
		text, _ := lvl.MarshalText()
		levels[i] = string(text)
	}

	return
}
