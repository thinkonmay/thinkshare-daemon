package backup

import (
	"math"
	"testing"
	"time"
)

func TestBackup(t *testing.T) {
	StartBackup(
		"C:/ideacrawler/boot_image",
		"C:/gpu2",
	)

	time.Sleep(time.Duration(math.MaxInt64))
}
