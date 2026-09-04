// Package postgres — модель данных и доступ к PostgreSQL. Имена полей и
// значения перечислений — из docs/03-data-model.md, менять их без правки
// документа нельзя.
package postgres

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // драйвер database/sql поверх pgx
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// DB — пул подключений к базе.
type DB struct {
	*sql.DB
}

// Open открывает пул и накатывает миграции. dsn — строка вида
// postgres://user:pass@host:5432/db?sslmode=disable
func Open(dsn string) (*DB, error) {
	handle, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("открыть базу: %w", err)
	}
	// Пул скромный: нагрузка одного пользователя, а держать сотню соединений
	// к Postgres дороже, чем открыть новое под редкий всплеск.
	handle.SetMaxOpenConns(10)
	handle.SetMaxIdleConns(5)
	handle.SetConnMaxLifetime(time.Hour)

	if err := handle.Ping(); err != nil {
		handle.Close()
		return nil, fmt.Errorf("база недоступна: %w", err)
	}

	db := &DB{handle}
	if err := db.Migrate(); err != nil {
		handle.Close()
		return nil, err
	}
	return db, nil
}

// rebind переводит плейсхолдеры `?` в нумерованные `$1, $2, …`, которых ждёт
// Postgres. Запросы в пакете написаны с `?` — так они читаются привычнее и не
// ломаются при вставке условия в середину. Внутри строковых литералов замена
// не делается: там `?` — обычный символ.
func rebind(query string) string {
	var out strings.Builder
	out.Grow(len(query) + 8)
	number := 0
	inString := false
	for i := 0; i < len(query); i++ {
		char := query[i]
		switch {
		case char == '\'':
			// Экранированная кавычка '' внутри литерала не закрывает его.
			if inString && i+1 < len(query) && query[i+1] == '\'' {
				out.WriteString("''")
				i++
				continue
			}
			inString = !inString
			out.WriteByte(char)
		case char == '?' && !inString:
			number++
			fmt.Fprintf(&out, "$%d", number)
		default:
			out.WriteByte(char)
		}
	}
	return out.String()
}

// Exec, Query и QueryRow перекрывают методы пула, чтобы вызывающий не думал
// о нумерации плейсхолдеров.
func (db *DB) Exec(query string, args ...any) (sql.Result, error) {
	return db.DB.Exec(rebind(query), args...)
}

func (db *DB) Query(query string, args ...any) (*sql.Rows, error) {
	return db.DB.Query(rebind(query), args...)
}

func (db *DB) QueryRow(query string, args ...any) *sql.Row {
	return db.DB.QueryRow(rebind(query), args...)
}

// Migrate накатывает недостающие миграции из backend/internal/postgres/migrations:
// файлы .sql по порядку плюс таблица применённых версий.
func (db *DB) Migrate() error {
	if _, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL
		)`); err != nil {
		return fmt.Errorf("таблица миграций: %w", err)
	}

	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("чтение миграций: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		var applied int
		if err := db.QueryRow(
			`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, name,
		).Scan(&applied); err != nil {
			return err
		}
		if applied > 0 {
			continue
		}
		body, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		if err := db.applyMigration(name, string(body)); err != nil {
			return err
		}
	}
	return nil
}

// applyMigration выполняет файл целиком в одной транзакции: либо миграция
// применилась вся, либо база осталась нетронутой.
func (db *DB) applyMigration(name, body string) error {
	tx, err := db.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// pgx работает через расширенный протокол и не принимает несколько команд
	// в одном Exec, поэтому файл разбивается на отдельные выражения.
	for _, statement := range splitStatements(body) {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("миграция %s: %w", name, err)
		}
	}
	if _, err := tx.Exec(
		`INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)`,
		name, UTCNow(),
	); err != nil {
		return err
	}
	return tx.Commit()
}

// splitStatements режет файл миграции по `;` вне строковых литералов
// и комментариев. Хватает для обычного DDL, который здесь и лежит.
func splitStatements(body string) []string {
	var statements []string
	var current strings.Builder
	inString := false
	for i := 0; i < len(body); i++ {
		char := body[i]
		switch {
		case char == '\'':
			if inString && i+1 < len(body) && body[i+1] == '\'' {
				current.WriteString("''")
				i++
				continue
			}
			inString = !inString
			current.WriteByte(char)
		case char == '-' && !inString && i+1 < len(body) && body[i+1] == '-':
			// Комментарий до конца строки — выбрасываем целиком.
			for i < len(body) && body[i] != '\n' {
				i++
			}
			current.WriteByte('\n')
		case char == ';' && !inString:
			if statement := strings.TrimSpace(current.String()); statement != "" {
				statements = append(statements, statement)
			}
			current.Reset()
		default:
			current.WriteByte(char)
		}
	}
	if statement := strings.TrimSpace(current.String()); statement != "" {
		statements = append(statements, statement)
	}
	return statements
}

// nullTime — nullable-время для модели.
func nullTime(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	moment := value.Time.UTC()
	return &moment
}

// timePtr — nullable-время для записи в базу.
func timePtr(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func stringFromNull(raw sql.NullString) string {
	if !raw.Valid {
		return ""
	}
	return raw.String
}

// nullString — пустая строка в nullable-колонке хранится как NULL.
func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// OpenSchema открывает базу в отдельной схеме и возвращает функцию, которая
// эту схему убирает. Нужен тестам: каждый прогон получает чистую схему в общей
// базе — это на порядок быстрее, чем поднимать базу на каждый тест.
// Продакшн-код им не пользуется, но и зависимости от testing здесь нет.
func OpenSchema(dsn, schema string) (*DB, func(), error) {
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("открыть базу: %w", err)
	}
	defer admin.Close()

	// Имя схемы приходит из кода, не от пользователя, но кавычки всё равно
	// ставим: иначе схема с заглавными буквами молча уедет в нижний регистр.
	quoted := `"` + strings.ReplaceAll(schema, `"`, `""`) + `"`
	if _, err := admin.Exec(`CREATE SCHEMA IF NOT EXISTS ` + quoted); err != nil {
		return nil, nil, fmt.Errorf("создать схему %s: %w", schema, err)
	}

	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	db, err := Open(dsn + separator + "search_path=" + schema)
	if err != nil {
		return nil, nil, err
	}
	drop := func() {
		db.Close()
		cleaner, err := sql.Open("pgx", dsn)
		if err != nil {
			return
		}
		defer cleaner.Close()
		_, _ = cleaner.Exec(`DROP SCHEMA IF EXISTS ` + quoted + ` CASCADE`)
	}
	return db, drop, nil
}
