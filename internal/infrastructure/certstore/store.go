package certstore

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io/fs"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/pan-daniel/line-proton/internal/domain/signing"
	"github.com/sassoftware/relic/v8/lib/certloader"
)

type Store struct {
	directory string
	clock     func() time.Time
}

func New(directory string, clock func() time.Time) Store {
	return Store{directory: directory, clock: clock}
}

func (store Store) Exists() bool {
	return isPresent(store.certificatePath()) && isPresent(store.keyPath())
}

func (store Store) LoadOrCreate(identity signing.Identity) (*certloader.Certificate, error) {
	if err := store.ensure(identity); err != nil {
		return nil, err
	}
	return certloader.LoadX509KeyPair(store.certificatePath(), store.keyPath())
}

func (store Store) Remove() error {
	return errors.Join(removeIfPresent(store.keyPath()), removeIfPresent(store.certificatePath()))
}

func (store Store) ensure(identity signing.Identity) error {
	if store.Exists() {
		return nil
	}
	return store.create(identity)
}

func (store Store) create(identity signing.Identity) error {
	pair, err := generate(identity, store.clock())
	if err != nil {
		return err
	}
	if err := store.Remove(); err != nil {
		return err
	}
	return store.persist(pair)
}

type pemPair struct {
	certificate []byte
	key         []byte
}

func generate(identity signing.Identity, now time.Time) (pemPair, error) {
	key, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		return pemPair{}, err
	}
	certificate := template(identity, now)
	certificateDER, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &key.PublicKey, key)
	if err != nil {
		return pemPair{}, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return pemPair{}, err
	}
	return pemPair{
		certificate: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER}),
		key:         pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
	}, nil
}

func (store Store) persist(pair pemPair) error {
	if err := os.MkdirAll(store.directory, 0o700); err != nil {
		return err
	}
	if err := writeAtomically(store.certificatePath(), pair.certificate, 0o644); err != nil {
		return err
	}
	return writeAtomically(store.keyPath(), pair.key, 0o600)
}

func writeAtomically(path string, content []byte, mode fs.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	if err := fill(temporary, content, mode); err != nil {
		return errors.Join(err, os.Remove(temporary.Name()))
	}
	return os.Rename(temporary.Name(), path)
}

func fill(file *os.File, content []byte, mode fs.FileMode) error {
	_, writeErr := file.Write(content)
	return errors.Join(writeErr, file.Chmod(mode), file.Sync(), file.Close())
}

func isPresent(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func template(identity signing.Identity, now time.Time) *x509.Certificate {
	return &x509.Certificate{
		SerialNumber: big.NewInt(now.UnixNano()),
		Subject:      pkix.Name{CommonName: identity.CommonName(), Organization: []string{identity.Organization()}},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(signing.CertificateLifetime),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
	}
}

func (store Store) keyPath() string {
	return filepath.Join(store.directory, "key.pem")
}

func (store Store) certificatePath() string {
	return filepath.Join(store.directory, "cert.pem")
}

func removeIfPresent(path string) error {
	err := os.Remove(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
