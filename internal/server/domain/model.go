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
	UserID    string
	Type      SecretType
	Title     string
	Payload   []byte
	Metadata  string
	Ref       string // ссылка на объект в MinIO (для BINARY)
	Version   int
	CreatedAt time.Time
	UpdatedAt time.Time
}
