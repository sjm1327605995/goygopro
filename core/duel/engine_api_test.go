package duel

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sjm1327605995/goygopro/core/data"
	"github.com/sjm1327605995/goygopro/ocgcore"
)

// TestEngineAPIMatchesReference verifies that the loaded ocgcore binary
// registers every Lua API captured from the reference Fluorohydride/ygopro
// engine in referenceEngineAPI.
func TestEngineAPIMatchesReference(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if err := data.DefaultDataManager.LoadDB(filepath.Join(root, "cards.cdb")); err != nil {
		t.Fatal(err)
	}
	if err := ocgcore.Init(
		ocgcore.WithRootPath(root),
		ocgcore.WithScriptDirectory(filepath.Join(root, "script")),
		ocgcore.WithCardReader(func(cardId uint32) *ocgcore.CardData {
			return data.DefaultDataManager.GetData(cardId)
		}),
	); err != nil {
		t.Skipf("ocgcore library not available: %v", err)
	}

	expected := referenceEngineAPI

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
		t.Fatalf("loaded ocgcore binary does not match reference API:\n%s", got)
	}
	t.Logf("loaded engine registers all %d reference Lua API functions", len(expected["Duel"])+len(expected["Card"])+len(expected["Group"])+len(expected["Effect"])+len(expected["Debug"]))
}
