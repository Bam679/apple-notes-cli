//go:build integration && darwin

package notes

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Bam679/apple-notes-cli/internal/agentcli"
)

// Writes only to a fresh folder agent-cli-test-<random> in the default
// account; the policy allows nothing else. Cleanup purges the folder and its
// notes (including from Recently Deleted).
func TestIntegrationSandboxFolder(t *testing.T) {
	ctx := context.Background()
	b := NewJXA()
	accs, err := b.Accounts(ctx)
	if err != nil || len(accs) == 0 {
		t.Fatalf("accounts: %v", err)
	}
	buf := make([]byte, 4)
	_, _ = rand.Read(buf)
	path := accs[0].Name + "/agent-cli-test-" + hex.EncodeToString(buf)
	allow := func() agentcli.Effective {
		return agentcli.Effective{Layers: []agentcli.Policy{{Version: 1,
			Operations: agentcli.OpRule{Allow: []string{agentcli.OpRead, agentcli.OpWrite}},
			Scopes: agentcli.ScopeRules{Read: agentcli.Rule{Allow: []string{path, path + "/*"}},
				Write: agentcli.Rule{Allow: []string{path, path + "/*"}}}}}}
	}
	pol := allow()
	cli := func(stdin string, args ...string) (int, string) {
		app := NewApp(b, "it")
		app.LoadPolicy = func() (agentcli.Effective, error) { return pol, nil }
		var out, errb bytes.Buffer
		code := app.Main(ctx, args, strings.NewReader(stdin), &out, &errb)
		if code != 0 {
			t.Logf("%v -> %d %s", args, code, errb.String())
		}
		return code, out.String()
	}
	if code, _ := cli("", "folder-create", path, "--confirm"); code != 0 {
		t.Fatal("folder-create")
	}
	t.Cleanup(func() {
		fs, _ := b.Folders(ctx)
		for _, f := range fs {
			if f.Path == path {
				if err := b.DeleteFolder(ctx, f.ID, true); err != nil {
					t.Errorf("cleanup: %v", err)
				}
			}
		}
	})
	if code, _ := cli("", "folder-create", path+"/sub", "--confirm"); code != 0 {
		t.Fatal("nested folder-create")
	}
	hostile := `it-two "quoted" 'x' $(id) <b>bold</b> & ` + "`tick`"
	in := `{"folder":"` + path + `","title":"it-one","body":"line1\nline2"}` + "\n" +
		`{"folder":"` + path + `","title":` + mustJSON(hostile) + `,"body":"needle-xyz"}`
	code, out := cli(in, "add", "-", "--confirm", "--json")
	if code != 0 {
		t.Fatal("add")
	}
	var added struct{ Items []map[string]any }
	if err := json.Unmarshal([]byte(out), &added); err != nil || len(added.Items) != 2 {
		t.Fatalf("add output %s", out)
	}
	id := added.Items[1]["id"].(string)
	if code, out = cli("", "get", id, "--json"); code != 0 || !strings.Contains(out, "needle-xyz") || !strings.Contains(out, "$(id) <b>bold</b> &") {
		t.Fatalf("get: %s", out)
	}
	if code, out = cli("", "list", "--folder", path); code != 0 || !strings.Contains(out, "it-one") {
		t.Fatalf("list: %s", out)
	}
	if code, out = cli("", "search", "NEEDLE-XYZ", "--body", "--folder", path); code != 0 || !strings.Contains(out, id) {
		t.Fatalf("search: %s", out)
	}
	if code, _ = cli("", "append", id, "--body", "appended-123", "--confirm"); code != 0 {
		t.Fatal("append")
	}
	if code, out = cli("", "get", id); code != 0 || !strings.Contains(out, "appended-123") {
		t.Fatalf("after append: %s", out)
	}
	if code, _ = cli("", "move", id, "--to", path+"/sub", "--confirm"); code != 0 {
		t.Fatal("move")
	}
	if code, out = cli("", "list", "--folder", path+"/sub", "--json"); code != 0 || !strings.Contains(out, id) {
		t.Fatalf("moved note not in sub: %s", out)
	}
	if code, _ = cli("", "folder-delete", path, "--confirm"); code != agentcli.ExitConflict {
		t.Fatal("non-empty folder-delete must conflict")
	}
	pol = agentcli.Effective{Layers: []agentcli.Policy{agentcli.DefaultPolicy()}}
	if code, _ = cli("", "delete", id, "--confirm"); code != agentcli.ExitPolicy {
		t.Fatal("policy-denied delete must exit 4")
	}
	if _, err := b.Get(ctx, id); err != nil {
		t.Fatal("denied delete removed the note")
	}
	pol = allow()
	if code, _ = cli("", "delete", id, "--confirm"); code != 0 {
		t.Fatal("delete")
	}
	_ = b.Delete(ctx, id, true) // purge from Recently Deleted
}

func mustJSON(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// TestIntegrationMarkdownShowcase writes one formatted note into the review
// folder iCloud/agent-cli-test-format and leaves it there for a visual check
// in the Notes UI. It only runs with AGENT_CLI_NOTES_SHOWCASE=1.
func TestIntegrationMarkdownShowcase(t *testing.T) {
	if os.Getenv("AGENT_CLI_NOTES_SHOWCASE") != "1" {
		t.Skip("set AGENT_CLI_NOTES_SHOWCASE=1 to write the showcase note")
	}
	ctx := context.Background()
	b := NewJXA()
	fs, err := b.Folders(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var folder Folder
	for _, f := range fs {
		if f.Path == "iCloud/agent-cli-test-format" {
			folder = f
		}
	}
	if folder.ID == "" {
		t.Skip("review folder iCloud/agent-cli-test-format does not exist")
	}
	md := "# Heading\n## Subheading\n### Small heading\n" +
		"Plain text with **bold**, *italic*, <u>underline</u>, ~~strike~~ and `inline code`.\n" +
		"<font color=\"#E03131\">Red</font>, <font color=\"#1C7ED6\">blue</font> and <span style=\"font-size: 28px\">large</span> text.\n\n" +
		"- Bullet\n- Bullet with **bold**\n  - Nested bullet\n1. First\n2. Second\n\n" +
		"| Item | Qty | Note |\n|---|---|---|\n| Apples | 3 | **fresh** |\n| Pears | 5 | <font color=\"#2F9E44\">ripe</font> |\n\n" +
		"```\nfunc main() {\n    fmt.Println(\"hi\")\n}\n```\n> A quote (rendered in italics)\n"
	n, err := b.Add(ctx, folder.ID, MarkdownHTML("16 Markdown showcase", md))
	if err != nil {
		t.Fatal(err)
	}
	img := filepath.Join(t.TempDir(), "swatch.png")
	if err := os.WriteFile(img, swatchPNG(), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Attach(ctx, n.ID, img); err != nil {
		t.Fatal(err)
	}
	got, _ := b.Get(ctx, n.ID)
	t.Logf("showcase note %s, %d attachment(s)", n.ID, got.Attachments)
}

// swatchPNG returns a 120x60 solid PNG.
func swatchPNG() []byte {
	var raw bytes.Buffer
	for y := 0; y < 60; y++ {
		raw.WriteByte(0)
		for x := 0; x < 120; x++ {
			raw.Write([]byte{28, 126, 214})
		}
	}
	var z bytes.Buffer
	w := zlib.NewWriter(&z)
	_, _ = w.Write(raw.Bytes())
	_ = w.Close()
	chunk := func(kind string, data []byte) []byte {
		var c bytes.Buffer
		_ = binary.Write(&c, binary.BigEndian, uint32(len(data)))
		c.WriteString(kind)
		c.Write(data)
		_ = binary.Write(&c, binary.BigEndian, crc32.ChecksumIEEE(append([]byte(kind), data...)))
		return c.Bytes()
	}
	var hdr bytes.Buffer
	_ = binary.Write(&hdr, binary.BigEndian, []any{uint32(120), uint32(60), uint8(8), uint8(2), uint8(0), uint8(0), uint8(0)})
	out := []byte("\x89PNG\r\n\x1a\n")
	out = append(out, chunk("IHDR", hdr.Bytes())...)
	out = append(out, chunk("IDAT", z.Bytes())...)
	return append(out, chunk("IEND", nil)...)
}
