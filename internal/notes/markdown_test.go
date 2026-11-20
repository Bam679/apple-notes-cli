package notes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Bam679/apple-notes-cli/internal/agentcli"
)

func TestMarkdownHTML(t *testing.T) {
	md := "# Big\n## Medium\n### Small\nText with **bold**, *italic*, _also italic_, ~~gone~~ and `code <b>`.\n" +
		"<u>under</u> <font color=\"#E03131\">red</font> <span style=\"font-size: 28px\">big</span> " +
		"<span style=\"background: url(x)\">bad</span> <script>x</script>\n\n" +
		"- one\n- two\n  - nested\n1. first\n2. second\n\n" +
		"| Name | Qty |\n|---|---:|\n| Apples | 3 |\n\n```\n  code & more\n```\n> quote\n---\n"
	got := MarkdownHTML("Title", md)
	for _, want := range []string{
		`<div><b><span style="font-size: 24px">Title</span></b></div>`,
		`<div><b><span style="font-size: 24px">Big</span></b></div>`,
		`<div><b><span style="font-size: 18px">Medium</span></b></div>`,
		`<div><b>Small</b></div>`,
		`<b>bold</b>`, `<i>italic</i>`, `<i>also italic</i>`, `<strike>gone</strike>`,
		`<font face="Courier">code &lt;b&gt;</font>`,
		`<u>under</u>`, `<font color="#E03131">red</font>`, `<span style="font-size: 28px">big</span>`,
		`&lt;span style=&#34;background: url(x)&#34;&gt;bad`, `&lt;script&gt;x&lt;/script&gt;`,
		`<ul><li>one</li><li>two</li><ul><li>nested</li></ul></ul><div><br></div><ol><li>first</li><li>second</li></ol>`,
		`<table><tbody><tr><td><b>Name</b></td><td><b>Qty</b></td></tr><tr><td>Apples</td><td>3</td></tr></tbody></table>`,
		`<div><font face="Courier">&nbsp;&nbsp;code &amp; more</font></div>`,
		`<div><i>quote</i></div>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s\nin %s", want, got)
		}
	}
	if strings.Contains(got, "<script") {
		t.Fatal("script tag survived")
	}
}

func FuzzMarkdownHTML(f *testing.F) {
	f.Add("# x\n- a\n| a | b |\n|---|---|\n<font color=\"red\">r</font>")
	f.Add("<span style=\"color: red\" onclick=\"x\">y</span><img src=x onerror=y>")
	f.Fuzz(func(t *testing.T, md string) {
		out := strings.ToLower(MarkdownHTML("", md))
		for _, bad := range []string{"<script", "<img", "<a ", "<iframe", "onclick=\"", "onerror=\"", "<style"} {
			if strings.Contains(out, bad) {
				t.Fatalf("%q produced %q", md, bad)
			}
		}
	})
}

func TestMarkdownAndAttachFlags(t *testing.T) {
	dir := t.TempDir()
	img := filepath.Join(dir, "chart.png")
	if err := os.WriteFile(img, []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}
	b := newFake()
	code, _, e := run(t, b, sandboxRW, "", "add", "--folder", "iCloud/Agent", "--title", "Plan", "--body", "**bold**", "--markdown", "--attach", img, "--confirm")
	if code != 0 || !strings.Contains(b.notes[4].HTML, "<b>bold</b>") || b.notes[4].Attachments != 1 {
		t.Fatalf("markdown add: %d %s %+v", code, e, b.notes[4])
	}
	if code, _, _ := run(t, b, sandboxRW, "", "append", "N2", "--body", "- item", "--markdown", "--attach", img, "--confirm"); code != 0 || !strings.Contains(b.notes[1].HTML, "<ul><li>item</li></ul>") {
		t.Fatal("markdown append")
	}
	if code, _, _ := run(t, b, sandboxRW, "", "update", "N2", "--body", "~~x~~", "--markdown", "--confirm"); code != 0 || !strings.Contains(b.notes[1].HTML, "<strike>x</strike>") {
		t.Fatal("markdown update")
	}
	for _, bad := range []string{filepath.Join(dir, "missing.png"), dir} {
		if code, _, _ := run(t, newFake(), sandboxRW, "", "add", "--folder", "iCloud/Agent", "--title", "x", "--attach", bad, "--dry-run"); code != agentcli.ExitUsage {
			t.Errorf("attach %s must be a usage error, got %d", bad, code)
		}
	}
	if code, _, _ := run(t, newFake(), sandboxRW, "", "append", "N2", "--body", "x", "--attach", dir, "--dry-run"); code != agentcli.ExitUsage {
		t.Error("append with a directory must be a usage error")
	}
	big := filepath.Join(dir, "big.bin")
	f, _ := os.Create(big)
	_ = f.Truncate(maxAttachment + 1)
	_ = f.Close()
	if _, err := attachments([]string{big}); err == nil {
		t.Error("oversized attachment accepted")
	}
	if MarkdownHTML("", "") != "<div><br></div>" {
		t.Error("empty markdown")
	}
}
