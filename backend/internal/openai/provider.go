// Package llm — адаптер языковой модели. Смена провайдера правит только этот
// файл (docs/04-decisions.md, решение №5).
package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"personalinbox/internal/exceptions"
	"strings"
	"time"

	"personalinbox/internal/core"
	"personalinbox/internal/sqlite"
)

// Request — всё, что уходит в модель об одном сообщении.
type Request struct {
	Criteria  string
	Sender    string
	Subject   string
	Body      string
	Source    string
	Overrides []sqlite.Override
}

// Result — строгий JSON от модели, разобранный в структуру.
type Result struct {
	Level       string
	Category    string
	Deadline    string
	NeedsReply  bool
	NeedsAction bool
	Summary     string
}

// Analyzer — то, что умеет оценить сообщение. Тесты подставляют свою реализацию.
type Analyzer interface {
	Analyze(Request) (Result, error)
}

const systemPrompt = "Ты помощник, который сортирует входящие сообщения по важности для одного человека. " +
	"Отвечай строго JSON по заданной схеме. Все текстовые поля — на русском языке, " +
	"независимо от языка исходного сообщения.\n" +
	"level — один из: CRITICAL (нельзя пропустить, есть жёсткий срок или риск), " +
	"HIGH (важно, требует внимания сегодня-завтра), NORMAL (обычное рабочее), " +
	"LOW (рассылки, уведомления сервисов, ничего не требуют).\n" +
	"category — 1–2 слова, например «Финансы», «Юридическое», «Найм». " +
	"Если тема непонятна, верни пустую строку.\n" +
	"deadline — срок словами из сообщения: «Сегодня, 18:00», «Пятница». " +
	"Если срока нет — пустая строка.\n" +
	"summary — 1–2 предложения о сути сообщения."

// responseSchema — structured output: всё, что не соответствует схеме,
// считается ошибкой вызова.
func responseSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required": []string{
			"level", "category", "deadline", "needs_reply", "needs_action", "summary",
		},
		"properties": map[string]any{
			"level":        map[string]any{"type": "string", "enum": sqlite.Levels},
			"category":     map[string]any{"type": "string"},
			"deadline":     map[string]any{"type": "string"},
			"needs_reply":  map[string]any{"type": "boolean"},
			"needs_action": map[string]any{"type": "boolean"},
			"summary":      map[string]any{"type": "string"},
		},
	}
}

// BuildUserPrompt собирает пользовательскую часть промпта: критерии, само
// сообщение и последние ручные исправления как обратную связь.
func BuildUserPrompt(request Request) string {
	criteria := strings.TrimSpace(request.Criteria)
	if criteria == "" {
		criteria = "— не заданы, оценивай на общих основаниях —"
	}
	body := request.Body
	if len([]rune(body)) > core.LLMMaxBodyChars {
		body = string([]rune(body)[:core.LLMMaxBodyChars])
	}
	parts := []string{
		"Критерии важности пользователя (свободный текст, не жёсткие правила):",
		criteria,
		"",
		"Источник: " + request.Source,
		"Отправитель: " + request.Sender,
		"Тема: " + request.Subject,
		"Текст:",
		body,
	}
	if len(request.Overrides) > 0 {
		parts = append(parts, "",
			"Ручные исправления пользователя по прошлым сообщениям "+
				"(тема → уровень, который он считает верным):")
		for _, override := range request.Overrides {
			parts = append(parts, fmt.Sprintf("- %s → %s", override.Subject, override.Level))
		}
	}
	return strings.Join(parts, "\n")
}

// OpenAI — вызов chat completions напрямую по HTTP: SDK ради одного запроса
// избыточен, а зависимостей у pet-проекта должно быть меньше.
type OpenAI struct {
	APIKey string
	Model  string
	// BaseURL позволяет подменить адрес в тестах.
	BaseURL string
	Client  *http.Client
}

// NewOpenAI собирает адаптер из настроек.
func NewOpenAI(cfg core.Config) *OpenAI {
	return &OpenAI{
		APIKey:  cfg.OpenAIKey,
		Model:   cfg.OpenAIModel,
		BaseURL: "https://api.openai.com/v1",
		Client:  &http.Client{Timeout: 60 * time.Second},
	}
}

// Analyze — один вызов модели на сообщение. Любой сбой — exceptions.ErrModelUnavailable.
func (o *OpenAI) Analyze(request Request) (Result, error) {
	if o.APIKey == "" {
		return Result{}, fmt.Errorf("%w: OPENAI_API_KEY не задан", exceptions.ErrModelUnavailable)
	}
	payload := map[string]any{
		"model": o.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": BuildUserPrompt(request)},
		},
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "analysis",
				"schema": responseSchema(),
				"strict": true,
			},
		},
		"temperature": 0,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", exceptions.ErrModelUnavailable, err)
	}

	base := o.BaseURL
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	httpRequest, err := http.NewRequest(http.MethodPost, base+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", exceptions.ErrModelUnavailable, err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Authorization", "Bearer "+o.APIKey)

	client := o.Client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	response, err := client.Do(httpRequest)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", exceptions.ErrModelUnavailable, err)
	}
	defer response.Body.Close()

	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", exceptions.ErrModelUnavailable, err)
	}
	if response.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("%w: провайдер ответил %d", exceptions.ErrModelUnavailable, response.StatusCode)
	}

	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &completion); err != nil || len(completion.Choices) == 0 {
		return Result{}, fmt.Errorf("%w: ответ провайдера не разобрать", exceptions.ErrModelUnavailable)
	}
	return ParseResponse(completion.Choices[0].Message.Content)
}

// ParseResponse разбирает ответ модели. Всё, что не соответствует схеме, —
// ошибка вызова (docs/00-product-spec.md, п. 6.2).
func ParseResponse(raw string) (Result, error) {
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		short := raw
		if len(short) > 200 {
			short = short[:200]
		}
		return Result{}, fmt.Errorf("%w: ответ модели не разобрать: %s", exceptions.ErrModelUnavailable, short)
	}

	level, _ := data["level"].(string)
	if !sqlite.Contains(sqlite.Levels, level) {
		return Result{}, fmt.Errorf("%w: неизвестный уровень: %v", exceptions.ErrModelUnavailable, data["level"])
	}
	text := map[string]string{}
	for _, key := range []string{"category", "deadline", "summary"} {
		value, ok := data[key].(string)
		if !ok {
			return Result{}, fmt.Errorf("%w: поле %s не строка", exceptions.ErrModelUnavailable, key)
		}
		text[key] = strings.TrimSpace(value)
	}
	flags := map[string]bool{}
	for _, key := range []string{"needs_reply", "needs_action"} {
		value, ok := data[key].(bool)
		if !ok {
			return Result{}, fmt.Errorf("%w: поле %s не булево", exceptions.ErrModelUnavailable, key)
		}
		flags[key] = value
	}
	return Result{
		Level:       level,
		Category:    text["category"],
		Deadline:    text["deadline"],
		NeedsReply:  flags["needs_reply"],
		NeedsAction: flags["needs_action"],
		Summary:     text["summary"],
	}, nil
}
