package server

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// ParseSize converts a human-readable size string (e.g., "1G", "512M") to bytes
// Supports B, K/KB, M/MB, G/GB, T/TB units (case insensitive)
func ParseSize(size string) (uint64, error) {
	if size == "" {
		return 0, errors.New("empty size string")
	}

	// Regular expression to match number and unit
	re := regexp.MustCompile(`^(\d+\.?\d*)\s*([KMGT]B?|B?)$`)
	matches := re.FindStringSubmatch(strings.ToUpper(size))

	if matches == nil {
		return 0, fmt.Errorf("invalid size format: %s", size)
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number: %s", matches[1])
	}

	var multiplier float64
	switch matches[2] {
	case "B", "":
		multiplier = 1
	case "K", "KB":
		multiplier = math.Pow(1024, 1)
	case "M", "MB":
		multiplier = math.Pow(1024, 2)
	case "G", "GB":
		multiplier = math.Pow(1024, 3)
	case "T", "TB":
		multiplier = math.Pow(1024, 4)
	default:
		return 0, fmt.Errorf("unsupported unit: %s", matches[2])
	}

	bytes := value * multiplier
	if bytes > math.MaxUint64 {
		return 0, errors.New("size exceeds maximum uint64 value")
	}

	return uint64(bytes), nil
}
