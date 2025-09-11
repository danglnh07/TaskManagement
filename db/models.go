package db

import (
	"time"

	"gorm.io/gorm"
)

type Role string
type OAuthProvider string
type Status string

const (
	User  Role = "user"
	Admin Role = "admin"

	Google OAuthProvider = "google"
	GitHub OAuthProvider = "github"

	Incomplete Status = "incomplete"
	Complete   Status = "complete"
	Cancel     Status = "cancel"
)

type Account struct {
	gorm.Model
	Username        string        `json:"username"`
	Email           string        `json:"email"`
	Role            Role          `json:"role"`
	OauthProvider   OAuthProvider `json:"oauth_provider"`
	OauthProviderID string        `json:"oauth_provider_id"`
	TokenVersion    int           `json:"token_version"`
	Tasks           []Task        `json:"tasks" gorm:"foreignKey:IssuerID"`
}

type Task struct {
	gorm.Model
	IssuerID    uint      `json:"issuer_id"`
	TaskName    string    `json:"name"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Deadline    time.Time `json:"deadline"`
	Status      Status    `json:"status"`
}
