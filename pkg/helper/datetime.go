package helper

import (
	"fmt"
	"time"
)

func ParseDatetime(v string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,          // Handles 2000-10-10T12:20:23Z or with offsets
		"2006-01-02T15:04:05", // 2000-10-10T12:20:23
		"2006-01-02T15:04",    // 2000-10-10T12:20
		"2006-01-02",          // 2000-10-10
		"2006-01",             // 2000-10
		"2006",                // 2000
	}

	var t time.Time
	var err error

	for _, layout := range layouts {
		t, err = time.Parse(layout, v)
		if err == nil {
			return t, nil
		}
	}

	// If no layouts matched, return the last error or a custom one
	return time.Time{}, fmt.Errorf("failed to parse datetime '%s': %w", v, err)
}
