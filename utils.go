package jpf

import (
	"errors"
	"fmt"
)

func wrap(err error, msg string, args ...any) error {
	return errors.Join(fmt.Errorf(msg, args...), err)
}
