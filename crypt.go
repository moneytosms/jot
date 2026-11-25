package jot

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"filippo.io/age"
	"filippo.io/age/armor"
)

// EnvKey points at an alternative age identity (key) file.
const EnvKey = "JOT_KEY"

// encryptedMarker in a journal directory means new writes are encrypted.
const encryptedMarker = ".encrypted"

// ErrNoKey means an encrypted file was found but the key file is missing.
var ErrNoKey = errors.New("encryption key not found")

// KeyPath returns the age identity file jot uses for encrypted journals:
// JOT_KEY, else age-key.txt next to the config file.
func KeyPath() string {
	if p := strings.TrimSpace(os.Getenv(EnvKey)); p != "" {
		return p
	}
	if c := ConfigPath(); c != "" {
		return filepath.Join(filepath.Dir(c), "age-key.txt")
	}
	return ""
}

var (
	keyMu    sync.Mutex
	keyPath  string
	keyCache *age.X25519Identity
)

// loadKey reads the identity file, creating a new key when create is set and
// none exists. The key is cached per path.
func loadKey(create bool) (*age.X25519Identity, error) {
	keyMu.Lock()
	defer keyMu.Unlock()
	path := KeyPath()
	if path == "" {
		return nil, ErrNoKey
	}
	if keyCache != nil && keyPath == path {
		return keyCache, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) && create {
		id, err := age.GenerateX25519Identity()
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, err
		}
		text := "# jot encryption key: back this file up; without it encrypted journals cannot be read\n" +
			"# public key: " + id.Recipient().String() + "\n" + id.String() + "\n"
		if err := writeFileAtomic(path, []byte(text), 0o600); err != nil {
			return nil, err
		}
		keyCache, keyPath = id, path
		return id, nil
	}
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrNoKey, path)
		}
		return nil, err
	}
	ids, err := age.ParseIdentities(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("key file %s: %w", path, err)
	}
	id, ok := ids[0].(*age.X25519Identity)
	if !ok {
		return nil, fmt.Errorf("key file %s: not an X25519 identity", path)
	}
	keyCache, keyPath = id, path
	return id, nil
}

// isCiphertext reports whether file data is an armored age file.
func isCiphertext(data []byte) bool {
	return bytes.HasPrefix(data, []byte(armor.Header))
}

// readDayFile reads a day file, decrypting it when it is encrypted.
func readDayFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil || !isCiphertext(data) {
		return data, err
	}
	id, err := loadKey(false)
	if err != nil {
		return nil, err
	}
	r, err := age.Decrypt(armor.NewReader(bytes.NewReader(data)), id)
	if err != nil {
		return nil, fmt.Errorf("decrypting %s: %w", path, err)
	}
	return io.ReadAll(r)
}

// openDayFile is readDayFile as a reader, for the line scanners.
func openDayFile(path string) (io.ReadCloser, error) {
	data, err := readDayFile(path)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

// encryptData armors plaintext for the given identity.
func encryptData(plain []byte, id *age.X25519Identity) ([]byte, error) {
	var buf bytes.Buffer
	aw := armor.NewWriter(&buf)
	w, err := age.Encrypt(aw, id.Recipient())
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(plain); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	if err := aw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// IsEncrypted reports whether a journal encrypts its day files.
func (j *Jot) IsEncrypted(name string) bool {
	_, err := os.Stat(filepath.Join(j.root, j.journalName(name), encryptedMarker))
	return err == nil
}

// writeDayFile atomically replaces a day file, encrypting it when the journal
// is encrypted.
func (j *Jot) writeDayFile(name, path string, plain []byte) error {
	if j.IsEncrypted(name) {
		id, err := loadKey(false)
		if err != nil {
			return err
		}
		if plain, err = encryptData(plain, id); err != nil {
			return err
		}
	}
	return writeFileAtomic(path, plain, 0o600)
}

// SetEncrypted encrypts (on) or decrypts (off) every day file of a journal
// and records the choice, creating the key on first use. It returns the
// number of files rewritten. Files already in the wanted state are skipped,
// so an interrupted run can simply be repeated.
func (j *Jot) SetEncrypted(name string, on bool) (int, error) {
	if err := j.checkJournal(name); err != nil {
		return 0, err
	}
	name = j.journalName(name)
	dir := filepath.Join(j.root, name)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return 0, fmt.Errorf("%w: %s", ErrNoJournal, name)
	}
	id, err := loadKey(on)
	if err != nil {
		return 0, err
	}
	days, err := j.Days(name)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, day := range days {
		path := filepath.Join(dir, day+".md")
		raw, err := os.ReadFile(path)
		if err != nil {
			return n, err
		}
		if isCiphertext(raw) == on {
			continue
		}
		plain, err := readDayFile(path)
		if err != nil {
			return n, err
		}
		out := plain
		if on {
			if out, err = encryptData(plain, id); err != nil {
				return n, err
			}
		}
		if err := writeFileAtomic(path, out, 0o600); err != nil {
			return n, err
		}
		n++
	}
	marker := filepath.Join(dir, encryptedMarker)
	if on {
		err = writeFileAtomic(marker, []byte("day files here are age-encrypted; key: "+KeyPath()+"\n"), 0o600)
	} else if err = os.Remove(marker); os.IsNotExist(err) {
		err = nil
	}
	if err != nil {
		return n, err
	}
	state := "decrypt "
	if on {
		state = "encrypt "
	}
	return n, j.sync(state + name)
}
