package client

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

var (
	DefaultHTTPClient = &http.Client{
		Timeout: 25 * time.Second,
	}
	DefaultUserAgent = "VerifiSci/1.0 (https://github.com/hossam1522/VerifiSci; mailto:verifisci@example.com)"
)

func SafeGet(url string, headers map[string]string, maxRetries int) ([]byte, error) {
	if maxRetries <= 0 {
		maxRetries = 3
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("User-Agent", DefaultUserAgent)
		req.Header.Set("Accept", "application/json")

		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := DefaultHTTPClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetries-1 {
				time.Sleep(time.Duration(math.Pow(2, float64(attempt))) * time.Second)
				continue
			}
			return nil, err
		}

		if resp.StatusCode == 429 {
			resp.Body.Close()
			lastErr = fmt.Errorf("429 Too Many Requests from %s", url)
			if attempt < maxRetries-1 {
				time.Sleep(time.Duration(math.Pow(2, float64(attempt))) * time.Second)
				continue
			}
			return nil, lastErr
		}

		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			resp.Body.Close()
			return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, resp.Status)
		}

		if resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP server error %d: %s", resp.StatusCode, resp.Status)
			if attempt < maxRetries-1 {
				time.Sleep(time.Duration(math.Pow(2, float64(attempt))) * time.Second)
				continue
			}
			return nil, lastErr
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		return body, nil
	}

	return nil, lastErr
}
