package internal

import (
	"fmt"
)

func WrapErrorAppend(err error, msg string) error {
	return fmt.Errorf("%s: %w", msg, err)
}
