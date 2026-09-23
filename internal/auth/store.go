package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User is a local identity record.
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"password_hash"`
	Role         string    `json:"role"` // user | admin
	CreatedAt    time.Time `json:"created_at"`
}

type userFile struct {
	CreatedAt string `json:"created_at"`
	Users     []User `json:"users"`
}

var storeMu sync.Mutex

func loadUsers(storeDir string) (*userFile, error) {
	path := filepath.Join(storeDir, "users.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &userFile{Users: nil}, nil
		}
		return nil, err
	}
	var f userFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse users.json: %w", err)
	}
	return &f, nil
}

func saveUsers(storeDir string, f *userFile) error {
	path := filepath.Join(storeDir, "users.json")
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// CreateUser adds a user. Returns error if email already exists.
func CreateUser(email, password, role string) (*User, error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	storeDir, err := storePath()
	if err != nil {
		return nil, err
	}
	if !dirExists(storeDir) {
		return nil, fmt.Errorf("auth store not initialized — run `paradox auth init`")
	}

	f, err := loadUsers(storeDir)
	if err != nil {
		return nil, err
	}
	for _, u := range f.Users {
		if u.Email == email {
			return nil, fmt.Errorf("user %q already exists", email)
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	if role == "" {
		role = "user"
	}
	u := User{
		ID:           fmt.Sprintf("usr_%d", time.Now().UnixNano()),
		Email:        email,
		PasswordHash: string(hash),
		Role:         role,
		CreatedAt:    time.Now().UTC(),
	}
	f.Users = append(f.Users, u)
	if err := saveUsers(storeDir, f); err != nil {
		return nil, err
	}
	return &u, nil
}

// Authenticate verifies email+password and returns the user.
func Authenticate(email, password string) (*User, error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	storeDir, err := storePath()
	if err != nil {
		return nil, err
	}
	f, err := loadUsers(storeDir)
	if err != nil {
		return nil, err
	}
	for i := range f.Users {
		u := &f.Users[i]
		if u.Email != email {
			continue
		}
		if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
			return nil, fmt.Errorf("invalid credentials")
		}
		return u, nil
	}
	return nil, fmt.Errorf("invalid credentials")
}

// FindUserByID returns a user by id.
func FindUserByID(id string) (*User, error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	storeDir, err := storePath()
	if err != nil {
		return nil, err
	}
	f, err := loadUsers(storeDir)
	if err != nil {
		return nil, err
	}
	for i := range f.Users {
		if f.Users[i].ID == id {
			u := f.Users[i]
			return &u, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

// ListUsers returns all users.
func ListUsers() ([]User, error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	storeDir, err := storePath()
	if err != nil {
		return nil, err
	}
	f, err := loadUsers(storeDir)
	if err != nil {
		return nil, err
	}
	return f.Users, nil
}
