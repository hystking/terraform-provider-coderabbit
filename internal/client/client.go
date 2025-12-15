package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is the CodeRabbit API client
type Client struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient creates a new CodeRabbit API client
func NewClient(apiKey, baseURL string) *Client {
	return &Client{
		APIKey:  apiKey,
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
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

// doRequest performs an HTTP request to the CodeRabbit API
func (c *Client) doRequest(method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
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
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
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

// GetGitUserID resolves a GitHub username to a numeric user ID
func (c *Client) GetGitUserID(githubID string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/users/"+githubID, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create GitHub API request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to perform GitHub API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return "", fmt.Errorf("GitHub user '%s' not found", githubID)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("GitHub API error (status %d)", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read GitHub API response: %w", err)
	}

	var user GitHubUserResponse
	if err := json.Unmarshal(respBody, &user); err != nil {
		return "", fmt.Errorf("failed to parse GitHub API response: %w", err)
	}

	return fmt.Sprintf("%d", user.ID), nil
}

// GetSeats retrieves all seat assignments
func (c *Client) GetSeats() (*SeatsResponse, error) {
	respBody, err := c.doRequest(http.MethodGet, "/seats/", nil)
	if err != nil {
		return nil, err
	}

	var seats SeatsResponse
	if err := json.Unmarshal(respBody, &seats); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &seats, nil
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
