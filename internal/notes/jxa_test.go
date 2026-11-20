package notes

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Bam679/apple-notes-cli/internal/osa"
)

func TestScriptIsStatic(t *testing.T) {
	if !strings.Contains(script, "const OPS") || strings.Contains(script, "%s") {
		t.Fatal("script must be a static OPS table")
	}
	for _, op := range []string{"accounts", "folders", "notes", "get", "add", "set_body", "move", "delete", "folder_create", "folder_delete"} {
		if !strings.Contains(script, "  "+op+": function") {
			t.Errorf("missing op %s", op)
		}
	}
}

func TestJXAWireMapping(t *testing.T) {
	ms := int64(1767225600000)
	wire := map[string]any{"id": "N", "title": "T", "folder": "iCloud/A", "folder_id": "F", "created_ms": ms, "modified_ms": nil, "locked": false, "attachments": 2, "text": "x", "html": "<div>x</div>"}
	seen := map[string]string{}
	f := &osa.Fake{Handler: func(op string, args json.RawMessage) (any, error) {
		seen[op] = string(args)
		switch op {
		case "accounts":
			return []Account{{ID: "A", Name: "iCloud"}}, nil
		case "folders":
			return []Folder{{ID: "F", Path: "iCloud/A"}}, nil
		case "folder_create":
			return Folder{ID: "F2", Path: "iCloud/B"}, nil
		case "notes":
			return []any{wire}, nil
		case "delete", "folder_delete":
			return nil, nil
		}
		return wire, nil
	}}
	j := &JXA{Runner: f}
	ctx := context.Background()
	if as, err := j.Accounts(ctx); err != nil || len(as) != 1 {
		t.Fatal(err)
	}
	if fs, err := j.Folders(ctx); err != nil || len(fs) != 1 {
		t.Fatal(err)
	}
	ns, err := j.Notes(ctx, []string{"F"}, true)
	if err != nil || len(ns) != 1 || ns[0].Attachments != 2 || !ns[0].Modified.IsZero() || ns[0].Created.UnixMilli() != ms {
		t.Fatalf("notes: %+v %v", ns, err)
	}
	if !strings.Contains(seen["notes"], `"with_text":true`) {
		t.Fatal(seen["notes"])
	}
	for _, call := range []func() error{
		func() error { _, err := j.Get(ctx, "N"); return err },
		func() error { _, err := j.Add(ctx, "F", "<div>x</div>"); return err },
		func() error { _, err := j.SetBody(ctx, "N", "<div>y</div>", true); return err },
		func() error { _, err := j.Move(ctx, "N", "F"); return err },
		func() error { return j.Delete(ctx, "N", false) },
		func() error { _, err := j.CreateFolder(ctx, "A", "", "B"); return err },
		func() error { return j.DeleteFolder(ctx, "F", false) },
	} {
		if err := call(); err != nil {
			t.Fatal(err)
		}
	}
	if !strings.Contains(seen["set_body"], `"append":true`) || NewJXA().Runner == nil {
		t.Fatal("set_body args")
	}
	fe := &osa.Fake{Handler: func(string, json.RawMessage) (any, error) { return nil, context.Canceled }}
	if _, err := (&JXA{Runner: fe}).Notes(ctx, nil, false); err == nil {
		t.Fatal("error must propagate")
	}
	if _, err := (&JXA{Runner: fe}).Get(ctx, "x"); err == nil {
		t.Fatal("error must propagate")
	}
}
