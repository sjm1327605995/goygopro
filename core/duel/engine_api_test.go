package duel

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sjm1327605995/goygopro/ocgcore"
)

// TestEngineAPIMatchesVendoredSource verifies that the LOADED ocgcore binary
// (prebuilt ocgcore.dll / libocgcore.so) registers exactly the Lua API that
// the vendored source/ygopro/ocgcore source declares. The repository ships a
// prebuilt engine binary, so a stale binary could silently lack functions that
// current card scripts call — the scripts would then fail at runtime with
// "attempt to call a nil value". This test closes that gap: it extracts every
// registered function name from libduel/libcard/libgroup/libeffect/libdebug
// and asks the live engine's Lua state whether each name resolves.
func TestEngineAPIMatchesVendoredSource(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if err := DefaultDataManager.LoadDB(filepath.Join(root, "cards.cdb")); err != nil {
		t.Fatal(err)
	}
	if err := ocgcore.Init(
		ocgcore.WithRootPath(root),
		ocgcore.WithScriptDirectory(filepath.Join(root, "script")),
		ocgcore.WithCardReader(func(cardId uint32) *ocgcore.CardData {
			return DefaultDataManager.GetData(cardId)
		}),
	); err != nil {
		t.Skipf("ocgcore library not available: %v", err)
	}

	// Extract "{ "Name", scriptlib::<prefix>..." registrations per namespace.
	type ns struct {
		table string // Lua global table name
		lib   string // source file
	}
	namespaces := []ns{
		{"Duel", "libduel.cpp"},
		{"Card", "libcard.cpp"},
		{"Group", "libgroup.cpp"},
		{"Effect", "libeffect.cpp"},
		{"Debug", "libdebug.cpp"},
	}
	re := regexp.MustCompile(`\{\s*"([A-Za-z0-9_]+)"\s*,\s*scriptlib::`)
	expected := map[string][]string{}
	for _, n := range namespaces {
		src, err := os.ReadFile(filepath.Join(root, "source", "ygopro", "ocgcore", n.lib))
		if err != nil {
			t.Fatalf("read %s: %v", n.lib, err)
		}
		for _, m := range re.FindAllStringSubmatch(string(src), -1) {
			expected[n.table] = append(expected[n.table], m[1])
		}
		if len(expected[n.table]) == 0 {
			t.Fatalf("extracted no %s functions from %s — regex out of date", n.table, n.lib)
		}
	}

	// Probe script: every expected name must resolve in the live Lua state.
	var b strings.Builder
	b.WriteString("local expect = {\n")
	for table, fns := range expected {
		fmt.Fprintf(&b, "\t%s = {", table)
		for _, f := range fns {
			fmt.Fprintf(&b, "%q,", f)
		}
		b.WriteString("},\n")
	}
	b.WriteString(`}
for tname, fns in pairs(expect) do
	local t = _G[tname]
	if t == nil then
		error("MISSING TABLE " .. tname)
	end
	for _, fname in ipairs(fns) do
		if t[fname] == nil then
			error("MISSING " .. tname .. "." .. fname)
		end
	end
end
Debug.Message("API_PROBE_OK")
`)
	probePath := filepath.Join(root, "script", "zz_api_probe.lua")
	if err := os.WriteFile(probePath, []byte(b.String()), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(probePath)

	d := ocgcore.NewDuel(1)
	if d == nil {
		t.Fatal("create duel: nil")
	}
	defer d.End()

	var got string
	d.SetErrorHandler(func(message string) { got += message + "\n" })

	rel := "./script/zz_api_probe.lua"
	if ocgcore.API.PreloadScript(d.GetNativePtr(), rel, int32(len(rel))) == 0 {
		t.Fatal("preload probe script failed")
	}
	if got == "" {
		// Some builds only flush the log on process(); nudge once.
		d.Process()
	}
	if !strings.Contains(got, "API_PROBE_OK") {
		t.Fatalf("loaded ocgcore binary does not match vendored source API:\n%s", got)
	}
	t.Logf("loaded engine registers all %d Lua API functions declared by vendored source", len(expected["Duel"])+len(expected["Card"])+len(expected["Group"])+len(expected["Effect"])+len(expected["Debug"]))
}
