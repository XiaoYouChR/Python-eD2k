package goed2k

import (
	"sync/atomic"
	"time"
)

var currentCachedTime atomic.Int64

func init() {
	currentCachedTime.Store(CurrentTimeHiRes())
}

func CurrentTime() int64 {
	return currentCachedTime.Load()
}

func UpdateCachedTime() {
	currentCachedTime.Store(CurrentTimeHiRes())
}

func CurrentTimeHiRes() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

func CurrentTimeToDeadline(nsFromNow int64) time.Time {
	return time.Now().Add(time.Duration(nsFromNow))
}

func CurrentTimeMillis() int64 {
	return time.Now().UnixMilli()
}

func Minutes(value int64) int64 {
	return value * 1000 * 60
}

func Hours(value int64) int64 {
	return value * 1000 * 3600
}

func Seconds(value int64) int64 {
	return value * 1000
}
