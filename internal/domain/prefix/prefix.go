package prefix

import (
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/pan-daniel/line-proton/internal/domain/signing"
)

const MarkerName = ".line-proton-signed"

var SystemDirectories = []string{"drive_c/windows/system32", "drive_c/windows/syswow64"}

var binaryExtensions = []string{".dll", ".drv", ".exe", ".acm", ".ax", ".ocx", ".cpl", ".tlb", ".ds", ".sys"}

func IsSystemBinary(name string) bool {
	return slices.Contains(binaryExtensions, strings.ToLower(filepath.Ext(name)))
}

type Kind int

const (
	RegularFile Kind = iota
	LinkIntoLineRunner
	LinkElsewhere
	BrokenLink
)

func (kind Kind) String() string {
	return [...]string{"regular file", "link into Proton-LINE", "link elsewhere", "broken link"}[kind]
}

type SystemFile struct {
	path     string
	kind     Kind
	modified time.Time
}

func NewSystemFile(path string, kind Kind, modified time.Time) SystemFile {
	return SystemFile{path: path, kind: kind, modified: modified}
}

func (file SystemFile) Path() string {
	return file.path
}

func (file SystemFile) Kind() Kind {
	return file.kind
}

func (file SystemFile) NeedsInspection() bool {
	return file.kind == RegularFile || file.kind == LinkElsewhere
}

type Action int

const (
	Keep Action = iota
	SignInPlace
	ReplaceLinkWithSignedCopy
	Skip
)

func Decide(file SystemFile, state signing.State) Action {
	if file.kind == BrokenLink || state == signing.NotSignable || state == signing.Damaged {
		return Skip
	}
	if state == signing.SignedByUs || file.kind == LinkIntoLineRunner {
		return Keep
	}
	if file.kind == LinkElsewhere {
		return ReplaceLinkWithSignedCopy
	}
	return SignInPlace
}

type Marker struct {
	signedAt time.Time
	exists   bool
}

func MarkedAt(signedAt time.Time) Marker {
	return Marker{signedAt: signedAt, exists: true}
}

func Unmarked() Marker {
	return Marker{}
}

func (marker Marker) CoversAll(files []SystemFile) bool {
	return marker.exists && !slices.ContainsFunc(files, marker.isStale)
}

func (marker Marker) isStale(file SystemFile) bool {
	return file.kind == LinkElsewhere || (file.kind == RegularFile && file.modified.After(marker.signedAt))
}

type Survey interface {
	Scan() ([]SystemFile, error)
	Marker() (Marker, error)
}

type Inventory interface {
	Survey
	Mark(signedAt time.Time) error
}

type LinkDetacher interface {
	DetachLink(path string) error
}

type FontInstaller interface {
	AddFonts(paths []string) error
}

type RegistryReader interface {
	ReadSystemRegistry() (string, error)
}

type Signable interface {
	Inventory
	LinkDetacher
}

type FontTarget interface {
	FontInstaller
	RegistryReader
}

type Contents interface {
	Exists(relativePath string) bool
}

type Purgeable interface {
	Contents
	Remove() error
}

type ProtonLock interface {
	Acquire() (func() error, error)
}
