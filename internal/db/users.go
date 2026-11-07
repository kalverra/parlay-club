package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrUserNotFound is returned when a user lookup matches no row.
var ErrUserNotFound = errors.New("user not found")

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
	Email       string
	PictureURL  string
}

// ListActiveUsers returns the users shown in the picker and member lists,
// ordered by name. Inactive users keep their history but drop out of both.
func (s *Store) ListActiveUsers(ctx context.Context) ([]User, error) {
	rows, err := s.conn().QueryContext(ctx, `
		SELECT id, name, avatar_color, home_book, active, email, picture_url
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

// CreateUserWithIdentity adds a user with a Google subject and email. Names may
// repeat: Google display names collide, and the sign-in flow does not rename an
// existing person to match one.
func (s *Store) CreateUserWithIdentity(
	ctx context.Context,
	name, email, googleSub, picture string,
) (User, error) {
	var user User
	err := s.Tx(ctx, func(tx *Store) error {
		var err error
		user, err = tx.insertUser(ctx, name, email, googleSub, picture)
		return err
	})
	if err != nil {
		return User{}, fmt.Errorf("create user %q: %w", name, err)
	}
	return user, nil
}

// insertUser writes one row with the next palette color. Empty identity values
// are stored as NULL, so an unlinked user has no google_sub.
func (s *Store) insertUser(ctx context.Context, name, email, googleSub, picture string) (User, error) {
	var count int
	if err := s.conn().QueryRowContext(ctx, "SELECT count(*) FROM users").Scan(&count); err != nil {
		return User{}, fmt.Errorf("count users: %w", err)
	}
	color := avatarPalette[count%len(avatarPalette)]

	result, err := s.conn().ExecContext(ctx, `
		INSERT INTO users (name, avatar_color, email, google_sub, picture_url)
		VALUES (?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''))`,
		name, string(color), email, googleSub, picture)
	if err != nil {
		return User{}, fmt.Errorf("insert user: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("insert user: %w", err)
	}

	user, err := s.UserByID(ctx, id)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

// UserByID returns the user with the given ID. A missing row wraps
// ErrUserNotFound. The Active flag comes back so callers can refuse a signed
// cookie for a user who has since been deactivated.
func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	user, err := scanUser(s.conn().QueryRowContext(ctx, `
		SELECT id, name, avatar_color, home_book, active, email, picture_url
		FROM users
		WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, fmt.Errorf("user %d: %w", id, ErrUserNotFound)
	}
	if err != nil {
		return User{}, fmt.Errorf("user %d: %w", id, err)
	}
	return user, nil
}

// UserByGoogleSub returns the user linked to an OpenID Connect subject. A
// missing row wraps ErrUserNotFound.
func (s *Store) UserByGoogleSub(ctx context.Context, sub string) (User, error) {
	user, err := scanUser(s.conn().QueryRowContext(ctx, `
		SELECT id, name, avatar_color, home_book, active, email, picture_url
		FROM users
		WHERE google_sub = ?`, sub))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, fmt.Errorf("user with google sub: %w", ErrUserNotFound)
	}
	if err != nil {
		return User{}, fmt.Errorf("user with google sub: %w", err)
	}
	return user, nil
}

// UserByEmail returns the user holding email, ignoring case. A missing row
// wraps ErrUserNotFound.
func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	user, err := scanUser(s.conn().QueryRowContext(ctx, `
		SELECT id, name, avatar_color, home_book, active, email, picture_url
		FROM users
		WHERE email = ?`, email))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, fmt.Errorf("user with email: %w", ErrUserNotFound)
	}
	if err != nil {
		return User{}, fmt.Errorf("user with email: %w", err)
	}
	return user, nil
}

// LinkGoogle attaches a Google subject, email, and picture to an existing user.
// A missing row wraps ErrUserNotFound.
func (s *Store) LinkGoogle(
	ctx context.Context,
	id int64,
	googleSub, email, picture string,
) (User, error) {
	result, err := s.conn().ExecContext(ctx, `
		UPDATE users
		SET google_sub = NULLIF(?, ''),
			email = NULLIF(?, ''),
			picture_url = NULLIF(?, '')
		WHERE id = ?`, googleSub, email, picture, id)
	if err != nil {
		return User{}, fmt.Errorf("link user %d: %w", id, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return User{}, fmt.Errorf("link user %d: %w", id, err)
	}
	if affected == 0 {
		return User{}, fmt.Errorf("link user %d: %w", id, ErrUserNotFound)
	}
	return s.UserByID(ctx, id)
}

// SetUserPicture refreshes one user's picture. An empty picture clears it. A
// missing row wraps ErrUserNotFound.
func (s *Store) SetUserPicture(ctx context.Context, id int64, picture string) (User, error) {
	result, err := s.conn().ExecContext(ctx,
		"UPDATE users SET picture_url = NULLIF(?, '') WHERE id = ?", picture, id)
	if err != nil {
		return User{}, fmt.Errorf("set picture for user %d: %w", id, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return User{}, fmt.Errorf("set picture for user %d: %w", id, err)
	}
	if affected == 0 {
		return User{}, fmt.Errorf("set picture for user %d: %w", id, ErrUserNotFound)
	}
	return s.UserByID(ctx, id)
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
		email    sql.NullString
		picture  sql.NullString
	)
	if err := row.Scan(
		&user.ID,
		&user.Name,
		&color,
		&homeBook,
		&user.Active,
		&email,
		&picture,
	); err != nil {
		return User{}, err
	}
	user.AvatarColor = AvatarColor(color)
	if homeBook.Valid {
		user.HomeBook = &homeBook.String
	}
	if email.Valid {
		user.Email = email.String
	}
	if picture.Valid {
		user.PictureURL = picture.String
	}
	return user, nil
}
