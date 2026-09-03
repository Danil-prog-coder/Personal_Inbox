package store

import (
	"database/sql"
	"database/sql/driver"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	sqlite "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// timeLayout — формат хранения даты в SQLite. Микросекунды пишутся всегда,
// иначе сравнение и сортировка по строке дали бы неверный порядок.
const timeLayout = "2006-01-02 15:04:05.000000"

// readLayouts — что умеем прочитать: свой формат и то, что мог записать
// прежний бэкенд на SQLAlchemy.
var readLayouts = []string{
	timeLayout,
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04:05",
}

var registerOnce sync.Once

// registerFunctions добавляет в SQLite регистронезависимый lower для кириллицы.
// Встроенный lower() умеет только латиницу — без замены поиск по русскому
// тексту был бы регистрозависимым.
func registerFunctions() {
	registerOnce.Do(func() {
		sqlite.MustRegisterDeterministicScalarFunction("unilower", 1,
			func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("unilower ждёт один аргумент")
				}
				switch value := args[0].(type) {
				case string:
					return strings.ToLower(value), nil
				case []byte:
					return strings.ToLower(string(value)), nil
				default:
					return args[0], nil
				}
			})
	})
}

// DB — подключение к базе. Одна база, один процесс, ничего сложнее не нужно.
type DB struct {
	*sql.DB
}

// Open открывает базу, включает WAL и внешние ключи и накатывает миграции.
func Open(path string) (*DB, error) {
	registerFunctions()

	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("каталог базы: %w", err)
		}
	}
	dsn := "file:" + path +
		"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	handle, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("открыть базу: %w", err)
	}
	// SQLite не любит параллельную запись, а нагрузка pet-проекта её и не требует.
	handle.SetMaxOpenConns(1)

	db := &DB{handle}
	if err := db.Migrate(); err != nil {
		handle.Close()
		return nil, err
	}
	return db, nil
}

// Migrate накатывает недостающие миграции из backend/internal/store/migrations.
// Заменяет Alembic из Python-версии: файлы .sql плюс таблица применённых версий.
func (db *DB) Migrate() error {
	if _, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at DATETIME NOT NULL
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
		if _, err := db.Exec(string(body)); err != nil {
			return fmt.Errorf("миграция %s: %w", name, err)
		}
		if _, err := db.Exec(
			`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			name, ToDBTime(UTCNow()),
		); err != nil {
			return err
		}
	}
	return nil
}

// ToDBTime приводит момент к формату хранения.
func ToDBTime(value time.Time) string {
	return value.UTC().Format(timeLayout)
}

// ToDBTimePtr — то же для nullable-колонок.
func ToDBTimePtr(value *time.Time) any {
	if value == nil {
		return nil
	}
	return ToDBTime(*value)
}

// FromDBTime разбирает то, что лежит в колонке DATETIME.
func FromDBTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	for _, layout := range readLayouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

// dbTime читает колонку DATETIME. Драйвер отдаёт такие колонки то строкой,
// то уже разобранным time.Time — принимаем оба варианта.
type dbTime struct {
	Time  time.Time
	Valid bool
}

// Scan разбирает значение колонки.
func (t *dbTime) Scan(src any) error {
	switch value := src.(type) {
	case nil:
		return nil
	case time.Time:
		t.Time, t.Valid = value.UTC(), true
	case string:
		parsed := FromDBTime(value)
		t.Time, t.Valid = parsed, !parsed.IsZero()
	case []byte:
		parsed := FromDBTime(string(value))
		t.Time, t.Valid = parsed, !parsed.IsZero()
	default:
		return fmt.Errorf("не время: %T", src)
	}
	return nil
}

// Ptr — nullable-время для модели.
func (t dbTime) Ptr() *time.Time {
	if !t.Valid {
		return nil
	}
	moment := t.Time
	return &moment
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
