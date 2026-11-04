package cmd

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// userRedactor keeps a user name fetched from the Secret Agent out of the log
// and the command's error. The zero value redacts nothing.
type userRedactor struct {
	replacer *strings.Replacer
	fetched  string
}

type redactedError struct {
	err  error
	text string
}

func (e *redactedError) Error() string {
	return e.text
}

func (e *redactedError) Unwrap() error {
	return e.err
}

func newUserRedactor(fetched string) userRedactor {
	r := userRedactor{
		fetched:  fetched,
		replacer: strings.NewReplacer(" `"+fetched+"`", "", fetched, ""),
	}
	logger.AddHook(r)

	return r
}

func (r userRedactor) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (r userRedactor) Fire(entry *logrus.Entry) error {
	entry.Message = r.replacer.Replace(entry.Message)

	for key, value := range entry.Data {
		if text := fmt.Sprint(value); strings.Contains(text, r.fetched) {
			entry.Data[key] = r.replacer.Replace(text)
		}
	}

	return nil
}

func (r userRedactor) redact(err error) error {
	if r.fetched == "" || err == nil || !strings.Contains(err.Error(), r.fetched) {
		return err
	}

	return &redactedError{err: err, text: r.replacer.Replace(err.Error())}
}
