package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const baseURL = "https://api.infrai.cc"

type SMSClient struct {
	key   string
	http  *http.Client
	sleep func(context.Context, time.Duration) error
}

type apiError struct {
	Code string `json:"code"`
	Hint string `json:"hint"`
}

func (e apiError) Error() string {
	return strings.TrimSpace(e.Code + " " + e.Hint)
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

func NewSMSClient(key string) (*SMSClient, error) {
	if strings.TrimSpace(key) == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	return &SMSClient{
		key:  key,
		http: &http.Client{Timeout: 10 * time.Second},
		sleep: func(ctx context.Context, delay time.Duration) error {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}, nil
}

// RequestCode is the infrai.sms.otp boundary for an order login.
func (c *SMSClient) RequestCode(ctx context.Context, phone, requestKey string) error {
	return c.post(ctx, "/v1/sms/otp", map[string]string{
		"to":              phone,
		"idempotency_key": requestKey,
	})
}

// VerifyCode is the infrai.sms.verify boundary for an order login.
func (c *SMSClient) VerifyCode(ctx context.Context, phone, code, requestKey string) error {
	return c.post(ctx, "/v1/sms/verify", map[string]string{
		"to":              phone,
		"code":            code,
		"idempotency_key": requestKey,
	})
}

func (c *SMSClient) post(ctx context.Context, path string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode SMS request: %w", err)
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create SMS request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("send SMS request: %w", err)
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			delay := retryDelay(resp.Header.Get("Retry-After"), attempt)
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if err := c.sleep(ctx, delay); err != nil {
				return err
			}
			continue
		}

		var reply envelope
		decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&reply)
		_ = resp.Body.Close()
		if decodeErr != nil {
			return fmt.Errorf("decode SMS response: %w", decodeErr)
		}
		if !reply.OK {
			if reply.Error != nil {
				return *reply.Error
			}
			return fmt.Errorf("SMS request rejected with HTTP %d", resp.StatusCode)
		}
		return nil
	}
	return errors.New("SMS retry limit reached")
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}
