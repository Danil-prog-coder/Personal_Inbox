package postgres

import "testing"

func TestRebindNumbersPlaceholders(t *testing.T) {
	cases := []struct{ in, want string }{
		{"SELECT 1", "SELECT 1"},
		{"SELECT * FROM users WHERE id = ?", "SELECT * FROM users WHERE id = $1"},
		{
			"UPDATE users SET theme = ?, density = ? WHERE id = ?",
			"UPDATE users SET theme = $1, density = $2 WHERE id = $3",
		},
		// Знак вопроса внутри литерала — обычный символ, а не плейсхолдер.
		{"SELECT ? WHERE note = 'как?'", "SELECT $1 WHERE note = 'как?'"},
		// Удвоенная кавычка не закрывает литерал.
		{"SELECT ? WHERE note = 'it''s ok?' AND id = ?", "SELECT $1 WHERE note = 'it''s ok?' AND id = $2"},
		// ESCAPE '\' из поиска по ленте не должен ломать разбор.
		{`WHERE lower(subject) LIKE ? ESCAPE '\' AND id = ?`, `WHERE lower(subject) LIKE $1 ESCAPE '\' AND id = $2`},
	}
	for _, c := range cases {
		if got := rebind(c.in); got != c.want {
			t.Errorf("rebind(%q)\n  получили %q\n  ожидали  %q", c.in, got, c.want)
		}
	}
}

func TestSplitStatements(t *testing.T) {
	body := `-- комментарий; с точкой с запятой
CREATE TABLE a (id BIGSERIAL PRIMARY KEY);

CREATE TABLE b (
    note TEXT NOT NULL DEFAULT 'точка; с запятой'
);
CREATE INDEX ix_a ON a (id)`

	statements := splitStatements(body)
	if len(statements) != 3 {
		t.Fatalf("ожидали три выражения, получили %d: %#v", len(statements), statements)
	}
	if want := "CREATE TABLE a (id BIGSERIAL PRIMARY KEY)"; statements[0] != want {
		t.Errorf("первое выражение: %q", statements[0])
	}
	// Точка с запятой внутри строкового литерала не режет выражение.
	if got := statements[1]; !contains(got, "'точка; с запятой'") {
		t.Errorf("литерал распался: %q", got)
	}
	if want := "CREATE INDEX ix_a ON a (id)"; statements[2] != want {
		t.Errorf("последнее выражение без ; потерялось: %q", statements[2])
	}
}

func TestSplitStatementsIgnoresEmptyInput(t *testing.T) {
	for _, body := range []string{"", "   \n\t ", "-- только комментарий\n", ";;;"} {
		if got := splitStatements(body); len(got) != 0 {
			t.Errorf("splitStatements(%q) вернул %#v", body, got)
		}
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
