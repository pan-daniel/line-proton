package linecdn

import (
	"crypto/x509"
	_ "embed"
	"errors"
	"os"

	"github.com/pan-daniel/line-proton/internal/domain/signing"
	"github.com/sassoftware/relic/v8/lib/authenticode"
)

//go:embed globalsign-roots.pem
var globalSignRoots []byte

var (
	ErrUnsigned   = errors.New("the LINE installer carries no Authenticode signature")
	ErrBrokenRoot = errors.New("the embedded GlobalSign root certificates do not parse")
)

func GlobalSignRoots() (*x509.CertPool, error) {
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(globalSignRoots) {
		return nil, ErrBrokenRoot
	}
	return roots, nil
}

type Verifier struct {
	roots *x509.CertPool
}

func NewVerifier(roots *x509.CertPool) Verifier {
	return Verifier{roots: roots}
}

func (verifier Verifier) VerifiedSigner(path string) (signing.Subject, error) {
	file, err := os.Open(path)
	if err != nil {
		return signing.Subject{}, err
	}
	defer file.Close()
	signatures, err := authenticode.VerifyPE(file, false)
	if err != nil {
		return signing.Subject{}, err
	}
	if len(signatures) == 0 {
		return signing.Subject{}, ErrUnsigned
	}
	if err := signatures[0].VerifyChain(verifier.roots, nil, x509.ExtKeyUsageCodeSigning); err != nil {
		return signing.Subject{}, err
	}
	leaf := signatures[0].Certificate.Subject
	return signing.NewSubject(leaf.CommonName, leaf.SerialNumber, leaf.Organization, leaf.Country), nil
}
