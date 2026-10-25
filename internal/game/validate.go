package game

import (
	"errors"
	"net/mail"
	"strconv"
	"strings"
)

// The 3.3.5 client takes at most 16 characters in each login box, and
// AzerothCore refuses passwords over 16 (MAX_PASS_STR).
const (
	MinUsername = 3
	MaxUsername = 16
	MaxPassword = 16
	MaxEmail    = 254
)

// A FieldError is a problem with one form field, said so a player can fix it.
type FieldError struct {
	Field   string
	Message string
}

func (e *FieldError) Error() string { return e.Message }

func fieldErr(field, msg string) error { return &FieldError{Field: field, Message: msg} }

// AsFieldError unwraps a FieldError.
func AsFieldError(err error) (*FieldError, bool) {
	var fe *FieldError
	ok := errors.As(err, &fe)
	return fe, ok
}

// Rules for usernames and passwords, from the site's settings.
type Rules struct {
	MinPassword int
	BotPrefix   string // playerbots' random bot accounts: RNDBOT0, RNDBOT1...
}

// Username checks a new account name: letters and digits, which is what the
// login screen and the core both handle without surprises.
func (r Rules) Username(name string) error {
	if len(name) < MinUsername || len(name) > MaxUsername {
		return fieldErr("username", "Usernames are 3 to 16 letters or digits.")
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return fieldErr("username", "Usernames can only use letters and digits.")
		}
	}
	// playerbots treats every account starting with its prefix as one of its
	// own: a player who got one would find their characters taken over.
	if r.BotPrefix != "" && strings.HasPrefix(strings.ToUpper(name), strings.ToUpper(r.BotPrefix)) {
		return fieldErr("username", "That name is reserved, please pick another.")
	}
	return nil
}

// Password checks a new password: printable ASCII without spaces, because
// the game uppercases it and the client limits it to 16 characters.
func (r Rules) Password(username, password string) error {
	min := r.MinPassword
	if min < 4 {
		min = 4
	}
	if min > MaxPassword {
		min = MaxPassword
	}
	if len(password) < min || len(password) > MaxPassword {
		return fieldErr("password", passwordLengthMessage(min))
	}
	for i := 0; i < len(password); i++ {
		if c := password[i]; c < 0x21 || c > 0x7e {
			return fieldErr("password", "Passwords can use letters, digits and symbols, but no spaces or accents.")
		}
	}
	if strings.EqualFold(username, password) {
		return fieldErr("password", "Your password can't be your username.")
	}
	return nil
}

func passwordLengthMessage(min int) string {
	return "Passwords are " + strconv.Itoa(min) + " to 16 characters (the game allows no more than 16)."
}

// Email checks an address and returns it in the form the core stores:
// uppercased (AccountMgr::CreateAccount does the same).
func Email(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" || len(addr) > MaxEmail {
		return "", fieldErr("email", "Please enter your email address.")
	}
	parsed, err := mail.ParseAddress(addr)
	if err != nil || parsed.Address != addr || parsed.Name != "" {
		return "", fieldErr("email", "That doesn't look like an email address.")
	}
	at := strings.LastIndexByte(addr, '@')
	domain := addr[at+1:]
	if at < 1 || !strings.Contains(domain, ".") || strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return "", fieldErr("email", "That doesn't look like an email address.")
	}
	for i := 0; i < len(addr); i++ {
		if addr[i] < 0x21 || addr[i] > 0x7e {
			return "", fieldErr("email", "Please use a plain email address (no spaces or accents).")
		}
	}
	return strings.ToUpper(addr), nil
}
