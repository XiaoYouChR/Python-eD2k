package main

import (
	"fmt"
	"strings"
)

func humanRate(bytesPerSecond int) string {
	value := float64(bytesPerSecond)
	units := []string{"B/s", "KB/s", "MB/s", "GB/s"}
	unit := units[0]
	for i := 1; i < len(units) && value >= 1024; i++ {
		value /= 1024
		unit = units[i]
	}
	if unit == "B/s" {
		return fmt.Sprintf("%d%s", bytesPerSecond, unit)
	}
	return fmt.Sprintf("%.1f%s", value, unit)
}

func percent(value, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return (float64(value) * 100.0) / float64(total)
}

func emptyFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func humanSize(bytes int64) string {
	value := float64(bytes)
	units := []string{"B", "KB", "MB", "GB", "TB"}
	unit := units[0]
	for i := 1; i < len(units) && value >= 1024; i++ {
		value /= 1024
		unit = units[i]
	}
	if unit == "B" {
		return fmt.Sprintf("%d%s", bytes, unit)
	}
	return fmt.Sprintf("%.1f%s", value, unit)
}
