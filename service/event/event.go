package event

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/danglnh07/TaskManagement/db"
	"github.com/danglnh07/TaskManagement/util"
)

type EventScheduler interface {
	CreateEvent(task *db.Task) error
	UpdateEvent(task *db.Task) error
}

type GoogleCalendarManager struct {
	queries *db.Queries
	config  *util.Config
}

func NewGoogleCalendarManager(queries *db.Queries, config *util.Config) EventScheduler {
	return &GoogleCalendarManager{
		queries: queries,
		config:  config,
	}
}

type TokenData struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiredAt    time.Time `json:"expired_at"`
}

func (calendar *GoogleCalendarManager) RefreshToken(issuerID uint) (string, error) {
	// Check if the access token is expired or not
	var token TokenData
	result := calendar.queries.DB.
		Table("accounts").
		Select("access_token, refresh_token, expired_at").Where("id = ?", issuerID).
		Scan(&token)

	if result.Error != nil {
		return "", result.Error
	}

	if token.ExpiredAt.Before(time.Now()) {
		// Refresh token
		payload := url.Values{}
		payload.Set("client_id", calendar.config.GoogleClientID)
		payload.Set("client_secret", calendar.config.GoogleClientSecret)
		payload.Set("refresh_token", token.RefreshToken)
		payload.Set("grant_type", "refresh_token")

		resp, err := http.PostForm("https://oauth2.googleapis.com/token", payload)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		var newToken struct {
			AccessToken string `json:"access_token"`
			ExpiresIn   int    `json:"expires_in"`
			TokenType   string `json:"token_type"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&newToken); err != nil {
			return "", err
		}

		// update stored access token
		token.AccessToken = newToken.AccessToken
		token.ExpiredAt = time.Now().Add(time.Second * time.Duration(newToken.ExpiresIn))
		result = calendar.queries.DB.
			Table("accounts").
			Where("id", issuerID).
			Updates(map[string]any{
				"access_token": token.AccessToken,
				"expired_at":   token.ExpiredAt,
			})
		if result.Error != nil {
			return "", result.Error
		}
	}

	return token.AccessToken, nil
}

func (calendar *GoogleCalendarManager) CreateEventData(task *db.Task) map[string]any {
	return map[string]any{
		"summary":     task.TaskName,
		"description": task.Description,
		"start": map[string]string{
			"dateTime": task.Deadline.Format(time.RFC3339),
			"timeZone": "UTC",
		},
		"end": map[string]string{
			"dateTime": task.Deadline.Add(30 * time.Minute).Format(time.RFC3339),
			"timeZone": "UTC",
		},
		"reminder": map[string]any{
			"useDefault": false,
			"overrides": []map[string]any{
				{"method": "email", "minutes": 15},
				{"method": "popup", "minutes": 10},
			},
		},
	}
}

func (calendar *GoogleCalendarManager) CreateEvent(task *db.Task) error {
	// Check if the access token is expired or not
	accessToken, err := calendar.RefreshToken(task.IssuerID)
	if err != nil {
		return err
	}

	// Create event
	event := calendar.CreateEventData(task)

	// Create and send request
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	url := "https://www.googleapis.com/calendar/v3/calendars/primary/events"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check for response status code
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create event: %s", string(b))
	}

	// Update event ID in database
	var eventResp struct {
		EventID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&eventResp); err != nil {
		return err
	}
	result := calendar.queries.DB.Table("tasks").Where("id", task.ID).Update("event_id", eventResp.EventID)
	if result.Error != nil {
		return err
	}

	return nil
}

func (calendar *GoogleCalendarManager) UpdateEvent(task *db.Task) error {
	// Check if the access token is expired or not
	accessToken, err := calendar.RefreshToken(task.IssuerID)
	if err != nil {
		return err
	}

	// Create event
	event := calendar.CreateEventData(task)

	// Create and send request
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/primary/events/%s", task.EventID)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check for response status code
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update event: %s", string(b))
	}

	return nil
}
