package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var (
	// ErrUserNotFound is returned when a user lookup matches no row.
	ErrUserNotFound = errors.New("user not found")
	// ErrNameTaken is returned when a new user's name matches an existing user.
	ErrNameTaken = errors.New("name taken")
)

// AvatarColor is a palette name stored in users.avatar_color.
type AvatarColor string

// avatarPalette is the fixed rotation new users draw from, in order.
var avatarPalette = []AvatarColor{
	"sky",
	"rose",
	"amber",
	"emerald",
	"violet",
	"orange",
	"teal",
	"pink",
}

// avatarClasses maps each palette color to the Tailwind classes that paint an
// avatar. The classes are literal strings so Tailwind's source scan compiles
// them.
var avatarClasses = map[AvatarColor]string{
	"sky":     "bg-sky-500",
	"rose":    "bg-rose-500",
	"amber":   "bg-amber-500",
	"emerald": "bg-emerald-500",
	"violet":  "bg-violet-500",
	"orange":  "bg-orange-500",
	"teal":    "bg-teal-500",
	"pink":    "bg-pink-500",
}

// Class returns the Tailwind classes that paint an avatar in the color. A name
// outside the palette (say, a hand-edited row) falls back to slate.
func (c AvatarColor) Class() string {
	if class, ok := avatarClasses[c]; ok {
		return class
	}
	return "bg-slate-600"
}

// User is one group member.
type User struct {
	ID          int64
	Name        string
	AvatarColor AvatarColor
	HomeBook    *string
	Active      bool
}

// ListActiveUsers returns the users shown in the picker and member lists,
// ordered by name. Inactive users keep their history but drop out of both.
func (s *Store) ListActiveUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, avatar_color, home_book, active
		FROM users
		WHERE active = 1
		ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list active users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	users := make([]User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("list active users: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list active users: %w", err)
	}
	return users, nil
}

// CreateUser adds an active user with the next palette color. The palette runs
// round robin over every stored user, inactive ones included, so a returning
// guest keeps a stable position in the rotation.
func (s *Store) CreateUser(ctx context.Context, name string) (User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, fmt.Errorf("create user %q: %w", name, err)
	}
	defer func() { _ = tx.Rollback() }()

	taken, err := nameTaken(ctx, tx, name)
	if err != nil {
		return User{}, fmt.Errorf("create user %q: %w", name, err)
	}
	if taken {
		return User{}, fmt.Errorf("create user %q: %w", name, ErrNameTaken)
	}

	var count int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM users").Scan(&count); err != nil {
		return User{}, fmt.Errorf("create user %q: %w", name, err)
	}
	color := avatarPalette[count%len(avatarPalette)]

	result, err := tx.ExecContext(ctx,
		"INSERT INTO users (name, avatar_color) VALUES (?, ?)", name, string(color))
	if err != nil {
		return User{}, fmt.Errorf("create user %q: %w", name, err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("create user %q: %w", name, err)
	}

	if err := tx.Commit(); err != nil {
		return User{}, fmt.Errorf("create user %q: %w", name, err)
	}

	return User{ID: id, Name: name, AvatarColor: color, Active: true}, nil
}

// UserByID returns the user with the given ID. A missing row wraps
// ErrUserNotFound. The Active flag comes back so callers can refuse a signed
// cookie for a user who has since been deactivated.
func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, avatar_color, home_book, active
		FROM users
		WHERE id = ?`, id)

	user, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, fmt.Errorf("user %d: %w", id, ErrUserNotFound)
	}
	if err != nil {
		return User{}, fmt.Errorf("user %d: %w", id, err)
	}
	return user, nil
}

// nameTaken reports whether a user already holds name, ignoring case.
func nameTaken(ctx context.Context, tx *sql.Tx, name string) (bool, error) {
	var taken bool
	err := tx.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT 1 FROM users WHERE name = ?)", name).Scan(&taken)
	return taken, err
}

// rowScanner is the Scan method shared by *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanUser reads the columns every user query selects, in order.
func scanUser(row rowScanner) (User, error) {
	var (
		user     User
		color    string
		homeBook sql.NullString
	)
	if err := row.Scan(&user.ID, &user.Name, &color, &homeBook, &user.Active); err != nil {
		return User{}, err
	}
	user.AvatarColor = AvatarColor(color)
	if homeBook.Valid {
		user.HomeBook = &homeBook.String
	}
	return user, nil
}
