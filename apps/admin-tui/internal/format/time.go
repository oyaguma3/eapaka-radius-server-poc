package format

import (
	"fmt"
	"time"
)

// RFC3339 はUnix秒をRFC3339形式にフォーマットする。
func RFC3339(unixSec int64) string {
	return time.Unix(unixSec, 0).UTC().Format(time.RFC3339)
}

// RFC3339Local はUnix秒をローカルタイムゾーンのRFC3339形式にフォーマットする。
func RFC3339Local(unixSec int64) string {
	return time.Unix(unixSec, 0).Local().Format(time.RFC3339)
}

// DateTime はUnix秒を "2006-01-02 15:04:05" 形式にフォーマットする。
func DateTime(unixSec int64) string {
	return time.Unix(unixSec, 0).Local().Format("2006-01-02 15:04:05")
}

// DateTimeShort はUnix秒を "01-02 15:04" 形式にフォーマットする。
func DateTimeShort(unixSec int64) string {
	return time.Unix(unixSec, 0).Local().Format("01-02 15:04")
}

// Duration は秒数を人間が読みやすい形式にフォーマットする。
// 例: 3661 -> "1h 1m 1s"
func Duration(seconds int64) string {
	if seconds < 0 {
		return "-"
	}

	d := time.Duration(seconds) * time.Second

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	secs := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, secs)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, secs)
	}
	return fmt.Sprintf("%ds", secs)
}

// DurationShort は秒数を短い形式にフォーマットする。
// 例: 3661 -> "1:01:01"
func DurationShort(seconds int64) string {
	if seconds < 0 {
		return "-"
	}

	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, secs)
	}
	return fmt.Sprintf("%d:%02d", minutes, secs)
}

// Elapsed は開始時刻からの経過時間を計算してフォーマットする。
func Elapsed(startUnixSec int64) string {
	elapsed := time.Now().Unix() - startUnixSec
	return Duration(elapsed)
}
