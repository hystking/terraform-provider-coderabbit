package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sync"
	"time"
)

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxRetries  int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	RetryableStatusCodes []int
}

// DefaultRetryConfig returns sensible default retry settings
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:  3,
		BaseDelay:   1 * time.Second,
		MaxDelay:    30 * time.Second,
		RetryableStatusCodes: []int{429, 500, 502, 503, 504},
	}
}

// Client is the CodeRabbit API client
type Client struct {
	APIKey      string
	BaseURL     string
	GitHubToken string
	HTTPClient  *http.Client
	RetryConfig RetryConfig

	// Cache for seats response (valid for single terraform run)
	seatsCache     *SeatsResponse
	seatsCacheMu   sync.RWMutex
	seatsCacheOnce sync.Once
}

// NewClient creates a new CodeRabbit API client
func NewClient(apiKey, baseURL, githubToken string) *Client {
	return &Client{
		APIKey:      apiKey,
		BaseURL:     baseURL,
		GitHubToken: githubToken,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		RetryConfig: DefaultRetryConfig(),
	}
}

// isRetryableStatus checks if the status code should trigger a retry
func (c *Client) isRetryableStatus(statusCode int) bool {
	for _, code := range c.RetryConfig.RetryableStatusCodes {
		if statusCode == code {
			return true
		}
	}
	return false
}

// calculateBackoff returns the delay for the given attempt using exponential backoff
func (c *Client) calculateBackoff(attempt int) time.Duration {
	delay := time.Duration(float64(c.RetryConfig.BaseDelay) * math.Pow(2, float64(attempt)))
	if delay > c.RetryConfig.MaxDelay {
		delay = c.RetryConfig.MaxDelay
	}
	return delay
}

// SeatUser represents a user in the seats response
type SeatUser struct {
	GitUserID    string `json:"git_user_id"`
	SeatAssigned bool   `json:"seat_assigned"`
}

// SeatsResponse represents the response from GET /seats/
type SeatsResponse struct {
	Users []SeatUser `json:"users"`
}

// AssignSeatRequest represents the request body for POST /seats/assign
type AssignSeatRequest struct {
	GitUserID string `json:"git_user_id"`
}

// UnassignSeatRequest represents the request body for POST /seats/unassign
type UnassignSeatRequest struct {
	GitUserID string `json:"git_user_id"`
}

// SuccessResponse represents a successful API response
type SuccessResponse struct {
	Success bool `json:"success"`
}

// ErrorResponse represents an error API response
type ErrorResponse struct {
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (e *ErrorResponse) Error() string {
	if len(e.Errors) > 0 {
		return e.Errors[0].Message
	}
	return "unknown error"
}

// GitHubUserResponse represents the response from GitHub API
type GitHubUserResponse struct {
	ID    int    `json:"id"`
	Login string `json:"login"`
}

// doRequest performs an HTTP request to the CodeRabbit API with retry logic
func (c *Client) doRequest(method, path string, body any) ([]byte, error) {
	var jsonBody []byte
	var err error

	if body != nil {
		jsonBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	var lastErr error
	for attempt := 0; attempt <= c.RetryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(c.calculateBackoff(attempt - 1))
		}

		var reqBody io.Reader
		if jsonBody != nil {
			reqBody = bytes.NewBuffer(jsonBody)
		}

		req, err := http.NewRequest(method, c.BaseURL+"/v1"+path, reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("x-coderabbitai-api-key", c.APIKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to perform request: %w", err)
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			continue
		}

		if c.isRetryableStatus(resp.StatusCode) {
			lastErr = fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
			continue
		}

		if resp.StatusCode >= 400 {
			var errResp ErrorResponse
			if err := json.Unmarshal(respBody, &errResp); err != nil {
				return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
			}
			return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, errResp.Error())
		}

		return respBody, nil
	}

	return nil, fmt.Errorf("request failed after %d retries: %w", c.RetryConfig.MaxRetries, lastErr)
}

// GetGitUserID resolves a GitHub username to a numeric user ID with retry logic
func (c *Client) GetGitUserID(githubID string) (string, error) {
	var lastErr error

	for attempt := 0; attempt <= c.RetryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(c.calculateBackoff(attempt - 1))
		}

		req, err := http.NewRequest(http.MethodGet, "https://api.github.com/users/"+githubID, nil)
		if err != nil {
			return "", fmt.Errorf("failed to create GitHub API request: %w", err)
		}

		req.Header.Set("Accept", "application/vnd.github+json")
		if c.GitHubToken != "" {
			req.Header.Set("Authorization", "Bearer "+c.GitHubToken)
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to perform GitHub API request: %w", err)
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read GitHub API response: %w", err)
			continue
		}

		if resp.StatusCode == 404 {
			return "", fmt.Errorf("GitHub user '%s' not found", githubID)
		}

		if c.isRetryableStatus(resp.StatusCode) {
			lastErr = fmt.Errorf("GitHub API error (status %d)", resp.StatusCode)
			continue
		}

		if resp.StatusCode >= 400 {
			return "", fmt.Errorf("GitHub API error (status %d)", resp.StatusCode)
		}

		var user GitHubUserResponse
		if err := json.Unmarshal(respBody, &user); err != nil {
			return "", fmt.Errorf("failed to parse GitHub API response: %w", err)
		}

		return fmt.Sprintf("%d", user.ID), nil
	}

	return "", fmt.Errorf("GitHub API request failed after %d retries: %w", c.RetryConfig.MaxRetries, lastErr)
}

// GetSeats retrieves all seat assignments (cached for the lifetime of the client)
func (c *Client) GetSeats() (*SeatsResponse, error) {
	// Check cache first with read lock
	c.seatsCacheMu.RLock()
	if c.seatsCache != nil {
		cached := c.seatsCache
		c.seatsCacheMu.RUnlock()
		return cached, nil
	}
	c.seatsCacheMu.RUnlock()

	// Fetch from API with write lock
	c.seatsCacheMu.Lock()
	defer c.seatsCacheMu.Unlock()

	// Double-check after acquiring write lock
	if c.seatsCache != nil {
		return c.seatsCache, nil
	}

	respBody, err := c.doRequest(http.MethodGet, "/seats/", nil)
	if err != nil {
		return nil, err
	}

	var seats SeatsResponse
	if err := json.Unmarshal(respBody, &seats); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	c.seatsCache = &seats
	return &seats, nil
}

// InvalidateSeatsCache clears the seats cache, forcing a fresh fetch on next GetSeats call
func (c *Client) InvalidateSeatsCache() {
	c.seatsCacheMu.Lock()
	defer c.seatsCacheMu.Unlock()
	c.seatsCache = nil
}

// AssignSeat assigns a seat to a user
func (c *Client) AssignSeat(gitUserID string) error {
	reqBody := AssignSeatRequest{GitUserID: gitUserID}
	respBody, err := c.doRequest(http.MethodPost, "/seats/assign", reqBody)
	if err != nil {
		return err
	}

	var success SuccessResponse
	if err := json.Unmarshal(respBody, &success); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !success.Success {
		return fmt.Errorf("seat assignment failed")
	}

	// Invalidate cache since seat state changed
	c.InvalidateSeatsCache()

	return nil
}

// UnassignSeat unassigns a seat from a user
func (c *Client) UnassignSeat(gitUserID string) error {
	reqBody := UnassignSeatRequest{GitUserID: gitUserID}
	respBody, err := c.doRequest(http.MethodPost, "/seats/unassign", reqBody)
	if err != nil {
		return err
	}

	var success SuccessResponse
	if err := json.Unmarshal(respBody, &success); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !success.Success {
		return fmt.Errorf("seat unassignment failed")
	}

	// Invalidate cache since seat state changed
	c.InvalidateSeatsCache()

	return nil
}

// HasSeat checks if a user has a seat assigned
func (c *Client) HasSeat(gitUserID string) (bool, error) {
	seats, err := c.GetSeats()
	if err != nil {
		return false, err
	}

	for _, user := range seats.Users {
		if user.GitUserID == gitUserID && user.SeatAssigned {
			return true, nil
		}
	}

	return false, nil
}
