package crypto

import (
	"sync"
	"time"
)

const masterKeyTTL = 15 * time.Minute

// Vault хранит мастер-пароль в памяти с автоматическим сбросом через TTL.
type Vault struct {
	mu             sync.Mutex
	masterPassword string
	expiresAt      time.Time
	timer          *time.Timer
}

// NewVault создаёт новый Vault.
func NewVault() *Vault {
	return &Vault{}
}

// Store сохраняет мастер-пароль и запускает таймер очистки.
func (v *Vault) Store(masterPassword string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.masterPassword = masterPassword
	v.expiresAt = time.Now().Add(masterKeyTTL)

	if v.timer != nil {
		v.timer.Stop()
	}
	v.timer = time.AfterFunc(masterKeyTTL, func() {
		v.Clear()
	})
}

// Get возвращает мастер-пароль если TTL не истёк.
// Возвращает пустую строку и false если ключ просрочен или не установлен.
func (v *Vault) Get() (string, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.masterPassword == "" || time.Now().After(v.expiresAt) {
		v.masterPassword = ""
		return "", false
	}
	return v.masterPassword, true
}

// Refresh продлевает TTL на ещё 15 минут (при каждом использовании).
func (v *Vault) Refresh() {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.masterPassword == "" {
		return
	}
	v.expiresAt = time.Now().Add(masterKeyTTL)

	if v.timer != nil {
		v.timer.Stop()
	}
	v.timer = time.AfterFunc(masterKeyTTL, func() {
		v.Clear()
	})
}

// Clear стирает мастер-пароль из памяти.
func (v *Vault) Clear() {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.masterPassword = ""
	if v.timer != nil {
		v.timer.Stop()
		v.timer = nil
	}
}

// IsAvailable проверяет, доступен ли мастер-пароль.
func (v *Vault) IsAvailable() bool {
	_, ok := v.Get()
	return ok
}
