package winreg_test

import (
	"slices"
	"testing"

	"github.com/pan-daniel/line-proton/internal/domain/winreg"
)

func TestKeyReturnsDefensiveCopies(test *testing.T) {
	key := winreg.NewKey(`HKEY_CURRENT_USER\X`, winreg.StringValue("a", "b"))
	values := key.Values()
	values[0] = winreg.StringValue("changed", "changed")
	if key.Values()[0].Name() != "a" {
		test.Error("key was mutated through Values()")
	}
}

func TestMultiStringKeepsItems(test *testing.T) {
	value := winreg.MultiStringValue("Noto Sans CJK JP", "NotoSansThai-Regular.ttf,Noto Sans Thai")
	if value.Kind() != winreg.MultiString || !slices.Equal(value.Items(), []string{"NotoSansThai-Regular.ttf,Noto Sans Thai"}) {
		test.Errorf("value = %v %v", value.Kind(), value.Items())
	}
}

func TestPatchKeepsKeyOrder(test *testing.T) {
	patch := winreg.NewPatch(winreg.NewKey("A"), winreg.NewKey("B"))
	if patch.Keys()[0].Path() != "A" || patch.Keys()[1].Path() != "B" {
		test.Error("key order changed")
	}
}
