package filecreated

import (
	"os"
	"syscall"
	"time"
)

func FileCreated(fname string) time.Time {
	finfo, _ := os.Stat(fname)
	stat_t := finfo.Sys().(*syscall.Stat_t)
	return timespecToTime(stat_t.Mtim)
}

func timespecToTime(ts syscall.Timespec) time.Time {
	return time.Unix(int64(ts.Sec), int64(ts.Nsec))
}
