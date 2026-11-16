package linecdn_test

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/pan-daniel/line-proton/internal/infrastructure/linecdn"
	"github.com/sassoftware/relic/v8/lib/authenticode"
	"github.com/sassoftware/relic/v8/lib/certloader"
)

type authority struct {
	certificate *x509.Certificate
	key         *ecdsa.PrivateKey
}

func newKey(test *testing.T) *ecdsa.PrivateKey {
	test.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		test.Fatal(err)
	}
	return key
}

func issued(test *testing.T, template, parent *x509.Certificate, publicKey, signerKey crypto.PublicKey) *x509.Certificate {
	test.Helper()
	raw, err := x509.CreateCertificate(rand.Reader, template, parent, publicKey, signerKey)
	if err != nil {
		test.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(raw)
	if err != nil {
		test.Fatal(err)
	}
	return certificate
}

func newAuthority(test *testing.T) authority {
	test.Helper()
	key := newKey(test)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test Code Signing Root"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	return authority{certificate: issued(test, template, template, &key.PublicKey, key), key: key}
}

func lyCorporation() pkix.Name {
	return pkix.Name{CommonName: "LY Corporation", SerialNumber: "010401039979", Organization: []string{"LY Corporation"}, Country: []string{"JP"}}
}

func (issuer authority) codeSigner(test *testing.T, subject pkix.Name) *certloader.Certificate {
	test.Helper()
	key := newKey(test)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      subject,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
	}
	leaf := issued(test, template, issuer.certificate, &key.PublicKey, issuer.key)
	return &certloader.Certificate{Leaf: leaf, Certificates: []*x509.Certificate{leaf}, PrivateKey: key}
}

func (issuer authority) roots() *x509.CertPool {
	roots := x509.NewCertPool()
	roots.AddCert(issuer.certificate)
	return roots
}

func unsignedInstaller(test *testing.T) string {
	test.Helper()
	output := filepath.Join(test.TempDir(), "LineInst.exe")
	command := exec.CommandContext(test.Context(), "go", "build", "-o", output, "./testdata/hello")
	command.Env = append(os.Environ(), "GOOS=windows", "GOARCH=386", "CGO_ENABLED=0")
	if combined, err := command.CombinedOutput(); err != nil {
		test.Fatalf("build fixture: %v\n%s", err, combined)
	}
	return output
}

func signedInstaller(test *testing.T, signer *certloader.Certificate) string {
	test.Helper()
	path := unsignedInstaller(test)
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		test.Fatal(err)
	}
	defer file.Close()
	digest, err := authenticode.DigestPE(file, crypto.SHA256, false)
	if err != nil {
		test.Fatal(err)
	}
	patch, _, err := digest.Sign(context.Background(), signer, nil)
	if err != nil {
		test.Fatal(err)
	}
	if err := patch.Apply(file, path); err != nil {
		test.Fatal(err)
	}
	return path
}

func TestVerifiedSignerFollowsTheChainToATrustedRoot(test *testing.T) {
	issuer := newAuthority(test)
	installer := signedInstaller(test, issuer.codeSigner(test, lyCorporation()))
	signer, err := linecdn.NewVerifier(issuer.roots()).VerifiedSigner(installer)
	if err != nil || signer.String() != "CN=LY Corporation, serialNumber=010401039979, O=LY Corporation, C=JP" {
		test.Errorf("VerifiedSigner = %q, %v", signer, err)
	}
}

func TestVerifiedSignerRejectsAChainOutsideTheGlobalSignRoots(test *testing.T) {
	installer := signedInstaller(test, newAuthority(test).codeSigner(test, lyCorporation()))
	roots, err := linecdn.GlobalSignRoots()
	if err != nil {
		test.Fatal(err)
	}
	if signer, err := linecdn.NewVerifier(roots).VerifiedSigner(installer); err == nil {
		test.Errorf("self-made chain accepted, signer %q", signer)
	}
}

func TestVerifiedSignerRejectsUnsignedInstallers(test *testing.T) {
	if _, err := linecdn.NewVerifier(newAuthority(test).roots()).VerifiedSigner(unsignedInstaller(test)); err == nil {
		test.Error("unsigned installer accepted")
	}
}

func TestEmbeddedRootsAreTheGlobalSignCodeSigningAndTimestampRoots(test *testing.T) {
	raw, err := os.ReadFile("globalsign-roots.pem")
	if err != nil {
		test.Fatal(err)
	}
	names := []string{}
	for block, rest := pem.Decode(raw); block != nil; block, rest = pem.Decode(rest) {
		names = append(names, commonNameOf(test, block.Bytes))
	}
	if !slices.Equal(names, []string{"GlobalSign Code Signing Root R45", "GlobalSign"}) {
		test.Errorf("embedded roots = %q", names)
	}
}

func commonNameOf(test *testing.T, der []byte) string {
	test.Helper()
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		test.Fatal(err)
	}
	return certificate.Subject.CommonName
}
