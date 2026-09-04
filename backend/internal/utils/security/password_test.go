package security

import (
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	hash, err := HashPassword("qwerty12345")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("qwerty12345", hash) {
		t.Fatal("верный пароль не подошёл")
	}
	if VerifyPassword("другой пароль", hash) {
		t.Fatal("неверный пароль подошёл")
	}
}

func TestLongPasswordDoesNotCrash(t *testing.T) {
	long := strings.Repeat("я", 200)
	hash, err := HashPassword(long)
	if err != nil {
		t.Fatalf("длинный пароль не захешировался: %v", err)
	}
	if !VerifyPassword(long, hash) {
		t.Fatal("длинный пароль не проверяется")
	}
}

func TestBrokenHashIsNotAnError(t *testing.T) {
	if VerifyPassword("любой", "") {
		t.Fatal("пустой хеш не должен подходить")
	}
	if VerifyPassword("любой", "не хеш вовсе") {
		t.Fatal("битый хеш не должен подходить")
	}
}
