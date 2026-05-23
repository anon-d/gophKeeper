package crypto

import "strings"

// ValidateLuhn проверяет номер карты по алгоритму Луна.
// Пробелы и дефисы игнорируются.
func ValidateLuhn(number string) bool {
	// Убираем пробелы и дефисы
	number = strings.ReplaceAll(number, " ", "")
	number = strings.ReplaceAll(number, "-", "")

	if len(number) < 13 || len(number) > 19 {
		return false
	}

	sum := 0
	double := false

	for i := len(number) - 1; i >= 0; i-- {
		d := int(number[i] - '0')
		if d < 0 || d > 9 {
			return false // не цифра
		}

		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}

		sum += d
		double = !double
	}

	return sum%10 == 0
}
