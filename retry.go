package main

import (
	"context"
	"time"
)

func doTry(ctx context.Context, max int, fn func() error) error {
	if max < 1 {
		max = 1
	}
	var err error
	for i := 0; i < max; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err = fn()
		if err == nil {
			return nil
		}
		if i < max-1 {
			// 1,2,4 秒
			sec := 1 << i
			time.Sleep(time.Duration(sec) * time.Second)
		}
	}
	return err
}
