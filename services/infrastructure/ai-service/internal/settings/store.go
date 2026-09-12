package settings

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

type store struct {
	dir    string
	cipher cipher.AEAD
}

func newStore(dir, key string) (*store, error) {
	if dir == "" && key == "" {
		return nil, nil
	}
	raw, err := base64.StdEncoding.DecodeString(key)
	if dir == "" || err != nil || len(raw) != 32 {
		return nil, errors.New("AI settings storage configuration is invalid")
	}
	block, err := aes.NewCipher(raw)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return nil, errors.New("AI settings storage is unavailable")
	}
	if err = os.Chmod(dir, 0700); err != nil {
		return nil, err
	}
	return &store{dir: dir, cipher: aead}, nil
}
func (s *store) read() (*record, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, "settings.enc"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.New("AI settings read failed")
	}
	n := s.cipher.NonceSize()
	if len(data) < n {
		return nil, errors.New("AI settings integrity check failed")
	}
	plain, err := s.cipher.Open(nil, data[:n], data[n:], []byte("askxuan-ai-settings-v1"))
	if err != nil {
		return nil, errors.New("AI settings integrity check failed")
	}
	var r record
	if json.Unmarshal(plain, &r) != nil {
		return nil, errors.New("AI settings data is invalid")
	}
	return &r, nil
}
func (s *store) write(r record, expected int64) error {
	lock, err := os.OpenFile(filepath.Join(s.dir, "write.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return errors.New("配置存储不可写")
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	old, err := s.read()
	if err != nil {
		return err
	}
	var revision int64
	if old != nil {
		revision = old.Revision
	}
	if revision != expected {
		return ErrConflict
	}
	plain, err := json.Marshal(r)
	if err != nil {
		return err
	}
	nonce := make([]byte, s.cipher.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return err
	}
	data := s.cipher.Seal(nonce, nonce, plain, []byte("askxuan-ai-settings-v1"))
	temp, err := os.CreateTemp(s.dir, ".settings-*")
	if err != nil {
		return errors.New("配置存储不可写")
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	if err = temp.Chmod(0600); err != nil {
		return err
	}
	if _, err = temp.Write(data); err != nil {
		return err
	}
	if err = temp.Sync(); err != nil {
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	// The previous revision is also encrypted; a failed rename leaves the active file intact.
	if old != nil {
		previous, err := os.ReadFile(filepath.Join(s.dir, "settings.enc"))
		if err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(s.dir, "previous.enc"), previous, 0600); err != nil {
			return err
		}
	}
	if err = os.Rename(temp.Name(), filepath.Join(s.dir, "settings.enc")); err != nil {
		return err
	}
	if dir, e := os.Open(s.dir); e == nil {
		defer dir.Close()
		_ = dir.Sync()
	}
	return nil
}
