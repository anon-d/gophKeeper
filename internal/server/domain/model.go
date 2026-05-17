package domain

import "time"

type SecretType int

const (
	Unknown SecretType = iota
	Credentials
	Password
	Bankcard
	Text
	Binary
)

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	PassHash  string    `json:"password"`
	CreatedAt time.Time `json:"created_at"`
}

type Secret struct {
	ID        string
	Type      SecretType `json:"type"`
	Title     string     `json:"title"`
	Payload   []byte     `json:"payload"`
	Metadata  string
	Version   int
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
