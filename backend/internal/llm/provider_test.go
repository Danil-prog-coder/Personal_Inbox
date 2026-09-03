package llm

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"personalinbox/internal/config"
	"personalinbox/internal/store"
)

const validResponse = `{"level": "HIGH", "category": "Финансы", "deadline": "Пятница",
	"needs_reply": true, "needs_action": false, "summary": "Нужны документы."}`

func TestParseValidResponse(t *testing.T) {
	result, err := ParseResponse(validResponse)
	if err != nil {
		t.Fatalf("корректный ответ не разобрался: %v", err)
	}
	if result.Level != "HIGH" || result.Category != "Финансы" || result.Deadline != "Пятница" {
		t.Fatalf("поля разобраны неверно: %+v", result)
	}
	if !result.NeedsReply || result.NeedsAction {
		t.Fatalf("признаки разобраны неверно: %+v", result)
	}
}

func TestParseTrimsWhitespace(t *testing.T) {
	result, err := ParseResponse(`{"level": "LOW", "category": "  Рассылка  ",
		"deadline": " ", "needs_reply": false, "needs_action": false,
		"summary": "  Дайджест.  "}`)
	if err != nil {
		t.Fatal(err)
	}
	if result.Category != "Рассылка" || result.Deadline != "" || result.Summary != "Дайджест." {
		t.Fatalf("пробелы не обрезаны: %+v", result)
	}
}

func TestInvalidResponseIsUnavailable(t *testing.T) {
	cases := map[string]string{
		"не JSON":            "не json вовсе",
		"не объект":          `[1, 2, 3]`,
		"чужой уровень":      `{"level": "URGENT", "category": "", "deadline": "", "needs_reply": false, "needs_action": false, "summary": ""}`,
		"категория не текст": `{"level": "LOW", "category": 5, "deadline": "", "needs_reply": false, "needs_action": false, "summary": ""}`,
		"признак не булев":   `{"level": "LOW", "category": "", "deadline": "", "needs_reply": "да", "needs_action": false, "summary": ""}`,
		"нет поля":           `{"level": "LOW", "category": ""}`,
	}
	for name, raw := range cases {
		if _, err := ParseResponse(raw); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("случай «%s» должен быть ошибкой вызова, получено: %v", name, err)
		}
	}
}

func TestPromptContainsCriteriaAndMessage(t *testing.T) {
	prompt := BuildUserPrompt(Request{
		Criteria: "Важны договоры.",
		Sender:   "Анна <a@northline.io>",
		Subject:  "Договор",
		Body:     "Текст письма",
		Source:   "gmail",
	})
	for _, fragment := range []string{"Важны договоры.", "Анна <a@northline.io>", "Договор",
		"Текст письма", "gmail"} {
		if !strings.Contains(prompt, fragment) {
			t.Fatalf("в промпте нет фрагмента %q", fragment)
		}
	}
}

func TestPromptWithoutCriteriaSaysSo(t *testing.T) {
	prompt := BuildUserPrompt(Request{Subject: "Тема"})
	if !strings.Contains(prompt, "не заданы") {
		t.Fatal("пустые критерии должны проговариваться в промпте")
	}
}

func TestPromptTruncatesLongBody(t *testing.T) {
	body := strings.Repeat("я", config.LLMMaxBodyChars+500)
	prompt := BuildUserPrompt(Request{Body: body})
	if !strings.Contains(prompt, strings.Repeat("я", config.LLMMaxBodyChars)) {
		t.Fatal("текст обрезан раньше границы в 4000 символов")
	}
	if strings.Contains(prompt, strings.Repeat("я", config.LLMMaxBodyChars+1)) {
		t.Fatal("текст не обрезан по границе в 4000 символов")
	}
}

func TestPromptIncludesOverrides(t *testing.T) {
	prompt := BuildUserPrompt(Request{
		Overrides: []store.Override{{Subject: "Счёт", Level: "CRITICAL"}},
	})
	if !strings.Contains(prompt, "- Счёт → CRITICAL") {
		t.Fatalf("ручные исправления не попали в промпт:\n%s", prompt)
	}
}

func TestAnalyzeWithoutAPIKeyIsUnavailable(t *testing.T) {
	client := &OpenAI{Model: "gpt-4o-mini"}
	if _, err := client.Analyze(Request{}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("без ключа ожидалась ошибка вызова, получено: %v", err)
	}
}

func TestAnalyzeParsesProviderAnswer(t *testing.T) {
	var seen map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&seen)
		if r.Header.Get("Authorization") != "Bearer ключ" {
			t.Errorf("ключ не передан: %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		payload, _ := json.Marshal(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": validResponse}},
			},
		})
		w.Write(payload)
	}))
	defer server.Close()

	client := &OpenAI{APIKey: "ключ", Model: "gpt-4o-mini", BaseURL: server.URL}
	result, err := client.Analyze(Request{Subject: "Тема"})
	if err != nil {
		t.Fatalf("вызов не удался: %v", err)
	}
	if result.Level != "HIGH" {
		t.Fatalf("оценка разобрана неверно: %+v", result)
	}
	if seen["model"] != "gpt-4o-mini" {
		t.Fatalf("в запрос ушла не та модель: %v", seen["model"])
	}
}

func TestProviderFailureIsUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := &OpenAI{APIKey: "ключ", BaseURL: server.URL}
	if _, err := client.Analyze(Request{}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("отказ провайдера должен быть ошибкой вызова: %v", err)
	}
}
