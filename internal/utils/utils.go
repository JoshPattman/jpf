package utils

import (
	"errors"
	"fmt"
)

func Wrap(err error, msg string, args ...any) error {
	return errors.Join(fmt.Errorf(msg, args...), err)
}
