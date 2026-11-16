package signing

import (
	"slices"
	"strings"
	"time"
)

const CertificateLifetime = 20 * 365 * 24 * time.Hour

type Identity struct {
	commonName   string
	organization string
}

func ImpersonatedWindowsIdentity() Identity {
	return Identity{commonName: "Microsoft Windows", organization: "Microsoft Corporation"}
}

func (identity Identity) CommonName() string {
	return identity.commonName
}

func (identity Identity) Organization() string {
	return identity.organization
}

type Subject struct {
	commonName    string
	serialNumber  string
	organizations []string
	countries     []string
}

func NewSubject(commonName, serialNumber string, organizations, countries []string) Subject {
	return Subject{commonName: commonName, serialNumber: serialNumber, organizations: slices.Clone(organizations), countries: slices.Clone(countries)}
}

func (subject Subject) String() string {
	return "CN=" + subject.commonName + ", serialNumber=" + subject.serialNumber + ", O=" + strings.Join(subject.organizations, "+") + ", C=" + strings.Join(subject.countries, "+")
}

type Publisher struct {
	commonName      string
	corporateNumber string
	organization    string
	country         string
}

func LYCorporation() Publisher {
	return Publisher{commonName: "LY Corporation", corporateNumber: "010401039979", organization: "LY Corporation", country: "JP"}
}

func (publisher Publisher) Signed(signer Subject) bool {
	return signer.commonName == publisher.commonName &&
		signer.serialNumber == publisher.corporateNumber &&
		slices.Equal(signer.organizations, []string{publisher.organization}) &&
		slices.Equal(signer.countries, []string{publisher.country})
}

type State int

const (
	Unsigned State = iota
	SignedByUs
	SignedByOther
	NotSignable
	Damaged
)

func (state State) NeedsSigning() bool {
	return state == Unsigned || state == SignedByOther
}

func (state State) String() string {
	return [...]string{"unsigned", "signed by line-proton", "signed by someone else", "not signable", "damaged signature"}[state]
}

type Signer interface {
	Sign(path string) error
}

type Inspector interface {
	Inspect(path string) (State, error)
}

type PublisherVerifier interface {
	VerifiedSigner(path string) (Subject, error)
}
