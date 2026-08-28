package platform

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Server struct {
	Mux                *http.ServeMux
	Store              *Store
	LDAPSettingsTester LDAPSettingsTester
	LDAPAuthenticator  LDAPAuthenticator
	sessions           map[[32]byte]AuthSession
	sessionsMu         sync.Mutex
}

type actorContextKey struct{}

const SessionCookieName = "sy_platform_session"

type AuthSession struct {
	UserID    int64
	ExpiresAt time.Time
}

type APIResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}

type Page[T any] struct {
	Items    []T   `json:"items"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

func NewServer(store *Store) *Server {
	return &Server{
		Mux:                http.NewServeMux(),
		Store:              store,
		LDAPSettingsTester: TestLDAPSettingsConnection,
		LDAPAuthenticator:  AuthenticateLDAP,
		sessions:           make(map[[32]byte]AuthSession),
	}
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := r
		if strings.HasPrefix(r.URL.Path, "/api/v1/") && s.ldapAuthenticationEnabled() && !isPublicAuthPath(r.URL.Path) {
			userID, ok := s.authenticatedUserID(r)
			if !ok {
				Error(w, http.StatusUnauthorized, "AUTH_REQUIRED", "请先使用 LDAP 账号登录")
				return
			}
			request = r.WithContext(context.WithValue(r.Context(), actorContextKey{}, userID))
		}
		s.Mux.ServeHTTP(w, request)
		if isMutationMethod(r.Method) {
			if err := s.Store.PersistSnapshot(); err != nil {
				log.Printf("persist platform store snapshot failed: %v", err)
			}
		}
	})
}

func isPublicAuthPath(path string) bool {
	switch path {
	case "/api/v1/iam/auth/config", "/api/v1/iam/login", "/api/v1/iam/logout":
		return true
	default:
		return false
	}
}

func (s *Server) ldapAuthenticationEnabled() bool {
	s.Store.Lock()
	defer s.Store.Unlock()
	return s.Store.Settings.LDAPAuth.Enabled
}

func ActorID(r *http.Request, store *Store) int64 {
	if r != nil {
		if userID, ok := r.Context().Value(actorContextKey{}).(int64); ok && userID != 0 {
			return userID
		}
	}
	return store.CurrentActorID()
}

func (s *Server) CreateSession(userID int64, now time.Time) (string, time.Time, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	expiresAt := now.Add(8 * time.Hour)
	s.sessionsMu.Lock()
	s.pruneExpiredSessionsLocked(now)
	s.sessions[sha256.Sum256([]byte(token))] = AuthSession{UserID: userID, ExpiresAt: expiresAt}
	s.sessionsMu.Unlock()
	return token, expiresAt, nil
}

func (s *Server) DeleteSession(token string) {
	if token == "" {
		return
	}
	s.sessionsMu.Lock()
	delete(s.sessions, sha256.Sum256([]byte(token)))
	s.sessionsMu.Unlock()
}

func (s *Server) authenticatedUserID(r *http.Request) (int64, bool) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil || cookie.Value == "" {
		return 0, false
	}
	now := time.Now()
	hash := sha256.Sum256([]byte(cookie.Value))
	s.sessionsMu.Lock()
	session, ok := s.sessions[hash]
	if ok && !session.ExpiresAt.After(now) {
		delete(s.sessions, hash)
		ok = false
	}
	s.sessionsMu.Unlock()
	if !ok {
		return 0, false
	}
	s.Store.Lock()
	defer s.Store.Unlock()
	for _, user := range s.Store.Users {
		if user.ID == session.UserID && user.Status == "enabled" {
			return user.ID, true
		}
	}
	return 0, false
}

func (s *Server) pruneExpiredSessionsLocked(now time.Time) {
	for key, session := range s.sessions {
		if !session.ExpiresAt.After(now) {
			delete(s.sessions, key)
		}
	}
}

func isMutationMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(APIResponse{
		Code:      "OK",
		Message:   "success",
		Data:      data,
		RequestID: "req-local",
	})
}

func Error(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(APIResponse{
		Code:      code,
		Message:   message,
		RequestID: "req-local",
	})
}

func Decode(r *http.Request, dst any) error {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return err
	}
	return nil
}

func Method(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	return false
}

func PathID(path, prefix string) (int64, error) {
	trimmed := strings.TrimPrefix(path, prefix)
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" {
		return 0, errors.New("missing id")
	}
	parts := strings.Split(trimmed, "/")
	return strconv.ParseInt(parts[0], 10, 64)
}

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
