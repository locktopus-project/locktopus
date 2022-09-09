package internal

import (
	"fmt"
	"net/http"
	"time"
)

func CheckServerAvailability(url string, timeoutMs int, pollIntervalMs int) error {
	ticker := time.NewTicker(time.Duration(pollIntervalMs) * time.Millisecond)
	timeout := time.After(time.Duration(timeoutMs) * time.Millisecond)

	for {
		select {
		case <-timeout:
			return fmt.Errorf("server %s is not available. Timeout reached", url)
		default:
		}

		select {
		case <-ticker.C:
		case <-timeout:
			return fmt.Errorf("server %s is not available. Timeout reached", url)
		}

		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == 200 {
			return nil
		}
	}
}
