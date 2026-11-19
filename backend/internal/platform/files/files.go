package files

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"path"
	"strings"
	"time"
	"unicode"

	"github.com/Akrambahy/EduFlow/backend/internal/platform/database"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/storage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound        = errors.New("file not found")
	ErrTooLarge        = errors.New("file is too large")
	ErrUnsupportedType = errors.New("file type is not allowed")
	ErrEmpty           = errors.New("file is empty")
	ErrInUse           = errors.New("file is still referenced")
)

type kind struct {
	contentType string
	sniffed     []string
}

var allowed = map[string]kind{
	"pdf":  {"application/pdf", []string{"application/pdf"}},
	"docx": {"application/vnd.openxmlformats-officedocument.wordprocessingml.document", []string{"application/zip"}},
	"pptx": {"application/vnd.openxmlformats-officedocument.presentationml.presentation", []string{"application/zip"}},
	"xlsx": {"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", []string{"application/zip"}},
	"zip":  {"application/zip", []string{"application/zip"}},
	"txt":  {"text/plain; charset=utf-8", []string{"text/plain"}},
	"md":   {"text/markdown; charset=utf-8", []string{"text/plain"}},
	"png":  {"image/png", []string{"image/png"}},
	"jpg":  {"image/jpeg", []string{"image/jpeg"}},
	"jpeg": {"image/jpeg", []string{"image/jpeg"}},
}

func AllowedExtensions() []string {
	return []string{"pdf", "docx", "pptx", "xlsx", "zip", "txt", "md", "png", "jpg", "jpeg"}
}

type File struct {
	ID          string    `json:"id"`
	OwnerID     string    `json:"ownerId"`
	Name        string    `json:"name"`
	ContentType string    `json:"contentType"`
	Extension   string    `json:"extension"`
	Size        int64     `json:"size"`
	CreatedAt   time.Time `json:"createdAt"`
	storageKey  string
}

type Requester struct {
	UserID  string
	Roles   []string
	IsAdmin bool
}

type AccessRule func(ctx context.Context, r Requester, fileID string) (bool, error)

type Service struct {
	db      *pgxpool.Pool
	store   storage.Storage
	maxSize int64
	rules   []AccessRule
}

func NewService(db *pgxpool.Pool, store storage.Storage, maxSize int64) *Service {
	return &Service{db: db, store: store, maxSize: maxSize}
}

func (s *Service) MaxSize() int64 {
	return s.maxSize
}

func (s *Service) AddAccessRule(rule AccessRule) {
	s.rules = append(s.rules, rule)
}

func SanitizeName(name string) string {
	name = path.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '"' {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if len(name) > 255 {
		name = name[len(name)-255:]
	}
	if name == "" || name == "." || name == "/" {
		return "file"
	}
	return name
}

func extensionOf(name string) string {
	i := strings.LastIndex(name, ".")
	if i < 0 || i == len(name)-1 {
		return ""
	}
	return strings.ToLower(name[i+1:])
}

func Validate(name string, head []byte) (kind, string, error) {
	ext := extensionOf(name)
	k, ok := allowed[ext]
	if !ok {
		return kind{}, "", ErrUnsupportedType
	}
	sniffed := http.DetectContentType(head)
	for _, want := range k.sniffed {
		if strings.HasPrefix(sniffed, want) {
			return k, ext, nil
		}
	}
	return kind{}, "", ErrUnsupportedType
}

type countingReader struct {
	r     io.Reader
	n     int64
	limit int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	if c.n > c.limit {
		return n, ErrTooLarge
	}
	return n, err
}

func (s *Service) Upload(ctx context.Context, ownerID, filename string, body io.Reader) (*File, error) {
	name := SanitizeName(filename)
	head := make([]byte, 512)
	n, err := io.ReadFull(body, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, err
	}
	head = head[:n]
	if n == 0 {
		return nil, ErrEmpty
	}
	k, ext, err := Validate(name, head)
	if err != nil {
		return nil, err
	}

	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, err
	}
	key := time.Now().UTC().Format("2006/01") + "/" + hex.EncodeToString(keyBytes)

	hash := sha256.New()
	counter := &countingReader{r: io.MultiReader(bytes.NewReader(head), body), limit: s.maxSize}
	if err := s.store.Put(ctx, key, io.TeeReader(counter, hash)); err != nil {
		_ = s.store.Delete(ctx, key)
		if errors.Is(err, ErrTooLarge) || isMaxBytes(err) {
			return nil, ErrTooLarge
		}
		return nil, err
	}

	f := &File{OwnerID: ownerID, Name: name, ContentType: k.contentType, Extension: ext, Size: counter.n, storageKey: key}
	err = s.db.QueryRow(ctx, `
		INSERT INTO files (owner_id, original_name, content_type, extension, size_bytes, sha256, storage_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, created_at`,
		ownerID, name, k.contentType, ext, counter.n, hash.Sum(nil), key).Scan(&f.ID, &f.CreatedAt)
	if err != nil {
		_ = s.store.Delete(ctx, key)
		return nil, err
	}
	return f, nil
}

func isMaxBytes(err error) bool {
	var mbe *http.MaxBytesError
	return errors.As(err, &mbe)
}

func (s *Service) get(ctx context.Context, id string) (*File, error) {
	var f File
	err := s.db.QueryRow(ctx, `
		SELECT id, owner_id, original_name, content_type, extension, size_bytes, created_at, storage_key
		FROM files WHERE id = $1`, id).
		Scan(&f.ID, &f.OwnerID, &f.Name, &f.ContentType, &f.Extension, &f.Size, &f.CreatedAt, &f.storageKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &f, err
}

func (s *Service) Authorized(ctx context.Context, r Requester, id string) (*File, error) {
	f, err := s.get(ctx, id)
	if err != nil {
		return nil, err
	}
	if r.IsAdmin || f.OwnerID == r.UserID {
		return f, nil
	}
	for _, rule := range s.rules {
		ok, err := rule(ctx, r, id)
		if err != nil {
			return nil, err
		}
		if ok {
			return f, nil
		}
	}
	return nil, ErrNotFound
}

func (s *Service) Open(ctx context.Context, f *File) (io.ReadSeekCloser, error) {
	return s.store.Open(ctx, f.storageKey)
}

func (s *Service) Delete(ctx context.Context, r Requester, id string) error {
	f, err := s.get(ctx, id)
	if err != nil {
		return err
	}
	if !r.IsAdmin && f.OwnerID != r.UserID {
		return ErrNotFound
	}
	if _, err := s.db.Exec(ctx, `DELETE FROM files WHERE id = $1`, id); err != nil {
		if database.IsForeignKeyViolation(err) {
			return ErrInUse
		}
		return err
	}
	return s.store.Delete(ctx, f.storageKey)
}

func (s *Service) OwnedBy(ctx context.Context, db database.DBTX, fileID, userID string) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM files WHERE id = $1 AND owner_id = $2)`, fileID, userID).Scan(&ok)
	return ok, err
}
