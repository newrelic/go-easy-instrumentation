package pkg

import (
	"errors"
	"time"
)

func DoSomething() error {
	time.Sleep(200 * time.Millisecond)
	return errors.New("an error has occured while doing something")
}
