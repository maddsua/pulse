package utils

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

func ParseDuration(val string) (time.Duration, error) {

	if val = strings.TrimSpace(val); val == "" || val == "0" {
		return 0, nil
	}

	for _, next := range val {
		if next < '0' || next > '9' {
			return time.ParseDuration(val)
		}
	}

	seconds, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, err
	}

	if seconds < 0 {
		return 0, errors.New("invalid duration value")
	}

	return time.Duration(seconds) * time.Second, nil
}
