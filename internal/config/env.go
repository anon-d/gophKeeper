// Package config предоставляет утилиты для конфигурации приложения.
package config

import "os"

// EnvOrDefault возвращает значение переменной окружения key.
// Если переменная не задана или пуста, возвращает def.
func EnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
