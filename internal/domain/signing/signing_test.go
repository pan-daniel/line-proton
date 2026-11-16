package signing_test

import (
	"testing"

	"github.com/pan-daniel/line-proton/internal/domain/signing"
)

func TestStateNeedsSigning(test *testing.T) {
	examples := map[signing.State]bool{
		signing.Unsigned:      true,
		signing.SignedByOther: true,
		signing.SignedByUs:    false,
		signing.NotSignable:   false,
		signing.Damaged:       false,
	}
	for state, want := range examples {
		test.Run(state.String(), func(test *testing.T) {
			if got := state.NeedsSigning(); got != want {
				test.Errorf("NeedsSigning() = %v, want %v", got, want)
			}
		})
	}
}

func TestImpersonatedWindowsIdentity(test *testing.T) {
	identity := signing.ImpersonatedWindowsIdentity()
	if identity.CommonName() != "Microsoft Windows" || identity.Organization() != "Microsoft Corporation" {
		test.Errorf("identity = %q / %q", identity.CommonName(), identity.Organization())
	}
}

func TestLYCorporationRecognisesOnlyItsOwnSignature(test *testing.T) {
	examples := map[string]struct {
		signer signing.Subject
		want   bool
	}{
		"LY Corporation in Japan":        {signing.NewSubject("LY Corporation", "010401039979", []string{"LY Corporation"}, []string{"JP"}), true},
		"look-alike common name":         {signing.NewSubject("LY Corporation Fake", "010401039979", []string{"LY Corporation"}, []string{"JP"}), false},
		"another organization":           {signing.NewSubject("LY Corporation", "010401039979", []string{"Evil Corp"}, []string{"JP"}), false},
		"an extra organization":          {signing.NewSubject("LY Corporation", "010401039979", []string{"LY Corporation", "Evil Corp"}, []string{"JP"}), false},
		"another country":                {signing.NewSubject("LY Corporation", "010401039979", []string{"LY Corporation"}, []string{"US"}), false},
		"another corporate number":       {signing.NewSubject("LY Corporation", "0104010399790", []string{"LY Corporation"}, []string{"JP"}), false},
		"no corporate number":            {signing.NewSubject("LY Corporation", "", []string{"LY Corporation"}, []string{"JP"}), false},
		"no organization and no country": {signing.NewSubject("LY Corporation", "010401039979", nil, nil), false},
	}
	for name, example := range examples {
		test.Run(name, func(test *testing.T) {
			if got := signing.LYCorporation().Signed(example.signer); got != example.want {
				test.Errorf("Signed(%v) = %v, want %v", example.signer, got, example.want)
			}
		})
	}
}

func TestSubjectDescribesItself(test *testing.T) {
	if got := signing.NewSubject("LY Corporation", "010401039979", []string{"LY Corporation"}, []string{"JP"}).String(); got != "CN=LY Corporation, serialNumber=010401039979, O=LY Corporation, C=JP" {
		test.Errorf("String = %q", got)
	}
}
