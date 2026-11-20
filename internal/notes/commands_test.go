package notes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Bam679/apple-notes-cli/internal/agentcli"
)

var update = flag.Bool("update", false, "rewrite golden files")

// TestMain pins the zone so golden files match on every machine.
func TestMain(m *testing.M) {
	time.Local = time.UTC
	os.Exit(m.Run())
}

// fakeBridge is an in-memory Bridge with synthetic data only.
type fakeBridge struct {
	accounts []Account
	folders  []Folder
	notes    []Note
	next     int
	fail     error
}

func newFake() *fakeBridge {
	t := func(d int) time.Time { return time.Date(2026, 11, d, 10, 0, 0, 0, time.UTC) }
	return &fakeBridge{
		accounts: []Account{{ID: "A1", Name: "iCloud"}},
		folders: []Folder{
			{ID: "F1", Name: "Notes", Path: "iCloud/Notes", Account: "iCloud"},
			{ID: "F2", Name: "Agent", Path: "iCloud/Agent", Account: "iCloud"},
			{ID: "F3", Name: "Private", Path: "iCloud/Private", Account: "iCloud"},
			{ID: "F4", Name: "Sub", Path: "iCloud/Agent/Sub", Account: "iCloud", ParentID: "F2"},
			{ID: "F5", Name: "Sub", Path: "iCloud/Notes/Sub", Account: "iCloud", ParentID: "F1"},
		},
		notes: []Note{
			{ID: "N1", Title: "Groceries", Folder: "iCloud/Notes", FolderID: "F1", Modified: t(10), Text: "milk\neggs"},
			{ID: "N2", Title: "Agent scratch", Folder: "iCloud/Agent", FolderID: "F2", Modified: t(12), Text: "todo: wifi setup"},
			{ID: "N3", Title: "Diary", Folder: "iCloud/Private", FolderID: "F3", Modified: t(14), Text: "secret"},
			{ID: "N4", Title: "Locked", Folder: "iCloud/Agent", FolderID: "F2", Modified: t(9), Locked: true},
		},
	}
}

func (f *fakeBridge) Accounts(context.Context) ([]Account, error) { return f.accounts, f.fail }
func (f *fakeBridge) Folders(context.Context) ([]Folder, error) {
	out := make([]Folder, len(f.folders))
	for i, fo := range f.folders {
		for _, n := range f.notes {
			if n.FolderID == fo.ID {
				fo.Notes++
			}
		}
		out[i] = fo
	}
	return out, f.fail
}

func (f *fakeBridge) Notes(_ context.Context, ids []string, withText bool) ([]Note, error) {
	var out []Note
	for _, n := range f.notes {
		if len(ids) > 0 && !has(ids, n.FolderID) {
			continue
		}
		if !withText {
			n.Text = ""
		}
		out = append(out, n)
	}
	return out, f.fail
}

func has(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

func (f *fakeBridge) idx(id string) int {
	for i, n := range f.notes {
		if n.ID == id {
			return i
		}
	}
	return -1
}

func (f *fakeBridge) folder(id string) Folder {
	for _, fo := range f.folders {
		if fo.ID == id {
			return fo
		}
	}
	return Folder{}
}

func (f *fakeBridge) Get(_ context.Context, id string) (Note, error) {
	if i := f.idx(id); i >= 0 {
		n := f.notes[i]
		if !n.Locked {
			n.HTML = "<div>" + n.Text + "</div>"
		} else {
			n.Text = ""
		}
		return n, nil
	}
	return Note{}, agentcli.NotFoundf("no note with id %s", id)
}

func (f *fakeBridge) Add(_ context.Context, folderID, html string) (Note, error) {
	if f.fail != nil {
		return Note{}, f.fail
	}
	f.next++
	fo := f.folder(folderID)
	n := Note{ID: fmt.Sprintf("NEW%d", f.next), Title: "new", Folder: fo.Path, FolderID: fo.ID, HTML: html}
	f.notes = append(f.notes, n)
	return n, nil
}

func (f *fakeBridge) SetBody(_ context.Context, id, html string, appendBody bool) (Note, error) {
	i := f.idx(id)
	if appendBody {
		f.notes[i].HTML += html
	} else {
		f.notes[i].HTML = html
	}
	return f.notes[i], nil
}

func (f *fakeBridge) Move(_ context.Context, id, folderID string) (Note, error) {
	i := f.idx(id)
	fo := f.folder(folderID)
	f.notes[i].FolderID, f.notes[i].Folder = fo.ID, fo.Path
	return f.notes[i], nil
}

func (f *fakeBridge) Attach(_ context.Context, id, path string) (Note, error) {
	i := f.idx(id)
	f.notes[i].Attachments++
	f.notes[i].HTML += "<attachment " + filepath.Base(path) + ">"
	return f.notes[i], nil
}

func (f *fakeBridge) Delete(_ context.Context, id string, _ bool) error {
	if f.fail != nil {
		return f.fail
	}
	i := f.idx(id)
	f.notes = append(f.notes[:i], f.notes[i+1:]...)
	return nil
}

func (f *fakeBridge) CreateFolder(_ context.Context, accountID, parentID, name string) (Folder, error) {
	path := "iCloud/" + name
	if parentID != "" {
		path = f.folder(parentID).Path + "/" + name
	}
	fo := Folder{ID: "F" + name, Name: name, Path: path, Account: "iCloud", ParentID: parentID}
	f.folders = append(f.folders, fo)
	return fo, nil
}

func (f *fakeBridge) DeleteFolder(_ context.Context, id string, force bool) error {
	for _, n := range f.notes {
		if n.FolderID == id && !force {
			return agentcli.Conflictf("folder is not empty")
		}
	}
	for i, fo := range f.folders {
		if fo.ID == id {
			f.folders = append(f.folders[:i], f.folders[i+1:]...)
			break
		}
	}
	return nil
}

var (
	readOnly  = agentcli.Effective{Layers: []agentcli.Policy{agentcli.DefaultPolicy()}}
	sandboxRW = agentcli.Effective{Layers: []agentcli.Policy{{Version: 1,
		Operations: agentcli.OpRule{Allow: []string{agentcli.OpRead, agentcli.OpWrite}},
		Scopes: agentcli.ScopeRules{Read: agentcli.Rule{Allow: []string{"*"}, Deny: []string{"iCloud/Private*"}},
			Write: agentcli.Rule{Allow: []string{"iCloud/Agent*"}}}}}}
)

func run(t *testing.T, b *fakeBridge, pol agentcli.Effective, stdin string, args ...string) (int, string, string) {
	t.Helper()
	app := NewApp(b, "test")
	app.LoadPolicy = func() (agentcli.Effective, error) { return pol, nil }
	app.Now = func() time.Time { return time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC) }
	var out, errb bytes.Buffer
	code := app.Main(context.Background(), args, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	p := filepath.Join("testdata", "golden", name)
	if *update {
		_ = os.MkdirAll(filepath.Dir(p), 0o750)
		if err := os.WriteFile(p, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(p) // #nosec G304 -- test fixture
	if err != nil {
		t.Fatalf("missing golden %s: run go test -update", p)
	}
	if got != string(want) {
		t.Fatalf("golden %s mismatch:\n%s\nwant:\n%s", name, got, want)
	}
}

func TestReadCommandsGolden(t *testing.T) {
	cases := map[string][]string{
		"accounts.txt":     {"accounts"},
		"folders.txt":      {"folders"},
		"list.txt":         {"list"},
		"list_folder.json": {"list", "--folder", "iCloud/Agent", "--json"},
		"list_window.txt":  {"list", "--modified-after", "2026-11-10", "--modified-before", "2026-11-12"},
		"search_body.txt":  {"search", "WIFI", "--body"},
		"get.txt":          {"get", "N2"},
		"get_html.json":    {"get", "N2", "--html", "--json"},
		"doctor.txt":       {"doctor"},
	}
	for name, args := range cases {
		code, out, errOut := run(t, newFake(), sandboxRW, "", args...)
		if code != 0 {
			t.Fatalf("%s: exit %d %s", name, code, errOut)
		}
		golden(t, name, out)
	}
}

func TestReadScopeHidesPrivate(t *testing.T) {
	for _, args := range [][]string{{"folders"}, {"list"}, {"search", "diary"}, {"search", "secret", "--body"}} {
		_, out, _ := run(t, newFake(), sandboxRW, "", args...)
		if strings.Contains(out, "Diary") || strings.Contains(out, "Private") {
			t.Fatalf("%v leaked private scope: %s", args, out)
		}
	}
	if code, _, _ := run(t, newFake(), sandboxRW, "", "get", "N3"); code != agentcli.ExitPolicy {
		t.Fatalf("get private: %d", code)
	}
	if code, _, _ := run(t, newFake(), sandboxRW, "", "list", "--folder", "iCloud/Private"); code != agentcli.ExitPolicy {
		t.Fatalf("list private: %d", code)
	}
}

func TestPolicyDeniedWriteIsBlocked(t *testing.T) {
	b := newFake()
	for _, args := range [][]string{
		{"add", "--folder", "iCloud/Agent", "--title", "x", "--confirm"},
		{"add", "--folder", "iCloud/Agent", "--title", "x", "--dry-run"},
		{"delete", "N2", "--confirm"},
		{"append", "N2", "--body", "x", "--confirm"},
	} {
		if code, _, e := run(t, b, readOnly, "", args...); code != agentcli.ExitPolicy {
			t.Fatalf("%v: exit %d %s", args, code, e)
		}
	}
	for _, args := range [][]string{
		{"delete", "N1", "--confirm"},
		{"move", "N2", "--to", "iCloud/Notes", "--confirm"},
		{"move", "N1", "--to", "iCloud/Agent", "--confirm"},
	} {
		if code, _, _ := run(t, b, sandboxRW, "", args...); code != agentcli.ExitPolicy {
			t.Fatalf("%v outside write scope: %d", args, code)
		}
	}
	if len(b.notes) != 4 || b.notes[1].Folder != "iCloud/Agent" {
		t.Fatal("a denied write changed data")
	}
}

func TestWriteFlow(t *testing.T) {
	b := newFake()
	if code, out, _ := run(t, b, sandboxRW, "", "add", "--folder", "iCloud/Agent", "--title", "T", "--body", "b", "--dry-run"); code != 0 || len(b.notes) != 4 || !strings.Contains(out, "true") {
		t.Fatalf("dry-run: %d %s", code, out)
	}
	if code, _, _ := run(t, b, sandboxRW, "", "add", "--folder", "iCloud/Agent", "--title", "T"); code != agentcli.ExitConfirm {
		t.Fatal("confirm required")
	}
	if code, _, e := run(t, b, sandboxRW, "", "add", "--folder", "F2", "--title", "<b>T</b>", "--body", "a\n\nb & c", "--confirm"); code != 0 {
		t.Fatalf("add: %s", e)
	}
	if got := b.notes[4].HTML; got != "<div><h1>&lt;b&gt;T&lt;/b&gt;</h1></div><div>a</div><div><br></div><div>b &amp; c</div>" {
		t.Fatalf("html escaping: %s", got)
	}
	in := `{"folder":"iCloud/Agent","title":"A"}` + "\n" + `{"folder":"iCloud/Agent/Sub","title":"B","body":"x"}`
	if code, _, e := run(t, b, sandboxRW, in, "add", "-", "--confirm"); code != 0 || len(b.notes) != 7 {
		t.Fatalf("stdin add: %d %s", code, e)
	}
	if code, _, _ := run(t, b, sandboxRW, "", "append", "N2", "--body", "more", "--confirm"); code != 0 || !strings.HasSuffix(b.notes[1].HTML, "<div>more</div>") {
		t.Fatal("append")
	}
	if code, _, _ := run(t, b, sandboxRW, "", "update", "N2", "--body", "new", "--confirm"); code != 0 || b.notes[1].HTML != "<div><h1>Agent scratch</h1></div><div>new</div>" {
		t.Fatalf("update keeps title: %s", b.notes[1].HTML)
	}
	if code, _, _ := run(t, b, sandboxRW, "", "update", "N2", "--title", "Renamed", "--body", "new", "--confirm"); code != 0 || !strings.Contains(b.notes[1].HTML, "Renamed") {
		t.Fatal("update title")
	}
	if code, _, e := run(t, b, sandboxRW, "", "move", "N2", "--to", "iCloud/Agent/Sub", "--confirm"); code != 0 || b.notes[1].FolderID != "F4" {
		t.Fatalf("move: %s", e)
	}
	if code, out, _ := run(t, b, sandboxRW, "", "delete", "N2", "--confirm", "--json"); code != 0 || !strings.Contains(out, `"deleted":true`) {
		t.Fatal("delete")
	}
	if code, _, e := run(t, b, sandboxRW, "", "folder-create", "iCloud/Agent2", "--confirm"); code != 0 {
		t.Fatalf("folder-create: %s", e)
	}
	if code, _, _ := run(t, b, sandboxRW, "", "folder-create", "iCloud/Agent/Deep", "--confirm"); code != 0 {
		t.Fatal("nested folder-create")
	}
	if code, _, _ := run(t, b, sandboxRW, "", "folder-create", "iCloud/Agent2", "--confirm"); code != agentcli.ExitConflict {
		t.Fatal("duplicate folder")
	}
	if code, _, _ := run(t, b, sandboxRW, "", "folder-delete", "iCloud/Agent2", "--confirm"); code != 0 {
		t.Fatal("folder-delete")
	}
	if code, _, _ := run(t, b, sandboxRW, "", "folder-delete", "iCloud/Agent", "--confirm"); code != agentcli.ExitConflict {
		t.Fatal("non-empty folder must not be deleted")
	}
}

func TestValidation(t *testing.T) {
	cases := map[string][]string{
		"no title":     {"add", "--folder", "iCloud/Agent", "--dry-run"},
		"bad arg":      {"add", "x", "--dry-run"},
		"empty append": {"append", "N2", "--body", "", "--dry-run"},
		"empty title":  {"update", "N2", "--title", " ", "--body", "x", "--dry-run"},
		"bad path":     {"folder-create", "NoSlash", "--dry-run"},
		"bad date":     {"list", "--modified-after", "x"},
		"no body":      {"update", "N2", "--dry-run"},
	}
	for name, args := range cases {
		if code, _, _ := run(t, newFake(), sandboxRW, "", args...); code != agentcli.ExitUsage {
			t.Errorf("%s: exit %d", name, code)
		}
	}
	if code, _, _ := run(t, newFake(), sandboxRW, "", "add", "--folder", "Sub", "--title", "x", "--dry-run"); code != agentcli.ExitConflict {
		t.Error("ambiguous folder name must conflict")
	}
	if code, _, _ := run(t, newFake(), sandboxRW, "", "add", "--folder", "Nope", "--title", "x", "--dry-run"); code != agentcli.ExitNotFound {
		t.Error("unknown folder")
	}
	if code, _, _ := run(t, newFake(), sandboxRW, "", "folder-create", "Nope/X", "--dry-run"); code != agentcli.ExitNotFound {
		t.Error("unknown account")
	}
	if code, _, _ := run(t, newFake(), sandboxRW, "", "append", "N4", "--body", "x", "--dry-run"); code != agentcli.ExitConflict {
		t.Error("locked note must not be modified")
	}
	if code, _, _ := run(t, newFake(), sandboxRW, "", "delete", "N2", "nope", "--dry-run"); code != agentcli.ExitNotFound {
		t.Error("unknown id")
	}
}

func TestBridgeErrors(t *testing.T) {
	b := newFake()
	b.fail = agentcli.Permissionf("denied")
	for _, args := range [][]string{{"accounts"}, {"folders"}, {"list"}, {"search", "x"}, {"list", "--folder", "x"}} {
		if code, _, _ := run(t, b, sandboxRW, "", args...); code != agentcli.ExitPermission {
			t.Errorf("%v: %d", args, code)
		}
	}
	if code, out, _ := run(t, b, sandboxRW, "", "doctor"); code != agentcli.ExitPermission || !strings.Contains(out, "false") {
		t.Errorf("doctor: %d %s", code, out)
	}
	b2 := newFake()
	b2.notes = append(b2.notes, Note{ID: "N9", Folder: "iCloud/Agent", FolderID: "F2"})
	b2.fail = nil
	code, _, _ := run(t, b2, sandboxRW, "", "delete", "N2", "N9", "--dry-run")
	if code != 0 {
		t.Fatal("dry-run batch")
	}
	b2.fail = agentcli.Unavailablef("x")
	if code, _, _ := run(t, b2, sandboxRW, "", "get", "N2"); code != 0 {
		t.Fatal("get does not use fail")
	}
	e := agentcli.AsError(partial(agentcli.Unavailablef("boom"), 1, 3))
	if !strings.HasPrefix(e.Message, "1 of 3") || !errors.Is(partial(e, 0, 1), e) {
		t.Fatal("partial")
	}
}

func TestNoteHTML(t *testing.T) {
	if NoteHTML("", "") != "" || BodyHTML("a\r\nb") != "<div>a</div><div>b</div>" {
		t.Fatal("html")
	}
}

func FuzzNoteHTML(f *testing.F) {
	f.Add("<script>alert(1)</script>", "a\nb")
	f.Fuzz(func(t *testing.T, title, body string) {
		out := NoteHTML(title, body)
		if strings.Contains(out, "<script") || strings.Contains(out, "<img") {
			t.Fatalf("unescaped markup: %q", out)
		}
	})
}

func TestSchemaWriteOps(t *testing.T) {
	_, out, _ := run(t, newFake(), readOnly, "", "schema")
	var s struct{ Commands []struct{ Kind string } }
	_ = json.Unmarshal([]byte(out), &s)
	n := 0
	for _, c := range s.Commands {
		if c.Kind == "write" {
			n++
		}
	}
	if n != 7 {
		t.Fatalf("write commands: %d", n)
	}
}
