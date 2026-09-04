// Package exceptions — сквозные ошибки приложения. Лежат отдельно, потому что
// их проверяют через errors.Is в слоях, которые друг о друге не знают:
// ручка не должна импортировать драйвер БД ради «не найдено».
package exceptions

import "errors"

// ErrNotFound — запрошенной строки нет в базе. Ручки превращают её в 404.
var ErrNotFound = errors.New("не найдено")

// ErrModelUnavailable — модель не ответила после всех повторов.
// Сообщение всё равно попадает в ленту, но с пометкой «Оценка недоступна».
var ErrModelUnavailable = errors.New("модель недоступна")

// ErrGmail и ErrTelegram — источник ответил ошибкой. Оборачиваются
// через %w, чтобы вызывающий отличил сбой источника от своей ошибки.
var (
	ErrGmail    = errors.New("gmail")
	ErrTelegram = errors.New("telegram")
)
