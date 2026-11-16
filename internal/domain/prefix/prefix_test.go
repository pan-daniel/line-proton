package prefix_test

import (
	"testing"
	"time"

	"github.com/pan-daniel/line-proton/internal/domain/prefix"
	"github.com/pan-daniel/line-proton/internal/domain/signing"
)

var noon = time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)

func TestDecide(test *testing.T) {
	regular := prefix.NewSystemFile("/p/system32/a.dll", prefix.RegularFile, noon)
	intoRunner := prefix.NewSystemFile("/p/system32/b.dll", prefix.LinkIntoLineRunner, noon)
	elsewhere := prefix.NewSystemFile("/p/system32/c.dll", prefix.LinkElsewhere, noon)
	broken := prefix.NewSystemFile("/p/system32/d.dll", prefix.BrokenLink, noon)
	examples := []struct {
		name  string
		file  prefix.SystemFile
		state signing.State
		want  prefix.Action
	}{
		{"signed regular file stays", regular, signing.SignedByUs, prefix.Keep},
		{"unsigned regular file is signed", regular, signing.Unsigned, prefix.SignInPlace},
		{"foreign signature is replaced", regular, signing.SignedByOther, prefix.SignInPlace},
		{"non PE file is skipped", regular, signing.NotSignable, prefix.Skip},
		{"damaged signature is skipped", regular, signing.Damaged, prefix.Skip},
		{"link into Proton-LINE stays", intoRunner, signing.Unsigned, prefix.Keep},
		{"unsigned foreign link becomes a signed copy", elsewhere, signing.Unsigned, prefix.ReplaceLinkWithSignedCopy},
		{"foreign link to our signed file stays", elsewhere, signing.SignedByUs, prefix.Keep},
		{"broken link is skipped", broken, signing.Unsigned, prefix.Skip},
	}
	for _, example := range examples {
		test.Run(example.name, func(test *testing.T) {
			if got := prefix.Decide(example.file, example.state); got != example.want {
				test.Errorf("Decide = %v, want %v", got, example.want)
			}
		})
	}
}

func TestOnlyRegularFilesAndForeignLinksNeedInspection(test *testing.T) {
	examples := map[prefix.Kind]bool{
		prefix.RegularFile:        true,
		prefix.LinkElsewhere:      true,
		prefix.LinkIntoLineRunner: false,
		prefix.BrokenLink:         false,
	}
	for kind, want := range examples {
		test.Run(kind.String(), func(test *testing.T) {
			if got := prefix.NewSystemFile("x.dll", kind, noon).NeedsInspection(); got != want {
				test.Errorf("NeedsInspection = %v, want %v", got, want)
			}
		})
	}
}

func TestMarkerCoversAll(test *testing.T) {
	marker := prefix.MarkedAt(noon)
	earlier := prefix.NewSystemFile("a.dll", prefix.RegularFile, noon.Add(-time.Minute))
	later := prefix.NewSystemFile("b.dll", prefix.RegularFile, noon.Add(time.Minute))
	newRunnerLink := prefix.NewSystemFile("c.dll", prefix.LinkIntoLineRunner, noon.Add(time.Minute))
	foreignLink := prefix.NewSystemFile("d.dll", prefix.LinkElsewhere, noon.Add(-time.Minute))
	brokenLink := prefix.NewSystemFile("e.dll", prefix.BrokenLink, noon.Add(time.Minute))
	examples := []struct {
		name   string
		marker prefix.Marker
		files  []prefix.SystemFile
		want   bool
	}{
		{"nothing changed", marker, []prefix.SystemFile{earlier, newRunnerLink}, true},
		{"file written after signing", marker, []prefix.SystemFile{earlier, later}, false},
		{"foreign link appeared", marker, []prefix.SystemFile{foreignLink}, false},
		{"broken link is not stale", marker, []prefix.SystemFile{earlier, brokenLink}, true},
		{"never signed", prefix.Unmarked(), []prefix.SystemFile{earlier}, false},
	}
	for _, example := range examples {
		test.Run(example.name, func(test *testing.T) {
			if got := example.marker.CoversAll(example.files); got != example.want {
				test.Errorf("CoversAll = %v, want %v", got, example.want)
			}
		})
	}
}

func TestIsSystemBinary(test *testing.T) {
	examples := map[string]bool{
		"gdiplus.dll":   true,
		"winealsa.DRV":  true,
		"notepad.exe":   true,
		"msacm32.acm":   true,
		"quartz.ax":     true,
		"hhctrl.ocx":    true,
		"joy.cpl":       true,
		"stdole2.tlb":   true,
		"sane.ds":       true,
		"mountmgr.sys":  true,
		"krnl386.exe16": false,
		"notes.txt":     false,
	}
	for name, want := range examples {
		test.Run(name, func(test *testing.T) {
			if got := prefix.IsSystemBinary(name); got != want {
				test.Errorf("IsSystemBinary(%q) = %v", name, got)
			}
		})
	}
}
