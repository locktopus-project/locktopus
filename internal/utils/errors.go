package internal

import "github.com/pkg/errors"

func WrapError(err error, msg string) error {
	return errors.Wrap(err, msg)
}

func WrapErrorString(err error, msg string) string {
	return errors.Wrapf(err, msg).Error()
}

func WrapErrorPrint(err error, msg string) {
	Logger.Error(WrapErrorString(err, msg))
}
