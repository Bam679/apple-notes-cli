package notes

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Bam679/apple-notes-cli/internal/agentcli"
)

var (
	folderFields = []string{"id", "path", "name", "account", "parent_id", "notes"}
	noteFields   = []string{"id", "title", "folder", "folder_id", "created", "modified", "locked", "attachments", "text", "html"}
	noteCols     = []string{"id", "modified", "title", "folder"}
)

// NewApp wires the CLI commands to bridge b.
func NewApp(b Bridge, version string) *agentcli.App {
	h := &handlers{b: b}
	folderFlag := agentcli.Flag{Name: "folder", Type: agentcli.Strings, Usage: "folder path (Account/Folder), name or id (repeatable)"}
	return &agentcli.App{
		Name:         "apple-notes",
		Version:      version,
		Summary:      "Apple Notes for AI agents (read-only unless the policy allows writes)",
		ScopeHelp:    "folder path \"Account/Folder/Subfolder\", e.g. \"iCloud/Agent\"",
		ScopeExample: "iCloud/Agent*",
		Commands: []*agentcli.Command{
			{Name: "accounts", Summary: "list Notes accounts", Kind: agentcli.Read,
				Fields: []string{"id", "name"}, AllFields: []string{"id", "name"}, Run: h.accounts},
			{Name: "folders", Summary: "list folders with full paths", Kind: agentcli.Read,
				Flags:  []agentcli.Flag{{Name: "account", Type: agentcli.String, Usage: "only this account"}},
				Fields: []string{"id", "path", "notes"}, AllFields: folderFields, Run: h.folders},
			{Name: "list", Summary: "list notes, newest first (no bodies)", Kind: agentcli.Read,
				Flags: []agentcli.Flag{folderFlag,
					{Name: "modified-after", Type: agentcli.String, Usage: "only notes modified on/after this date"},
					{Name: "modified-before", Type: agentcli.String, Usage: "only notes modified before this date"}},
				Fields: noteCols, AllFields: noteFields, Run: h.list, Example: "apple-notes list --folder iCloud/Notes --limit 20"},
			{Name: "search", Summary: "find notes by title (or --body: also plain-text content)", Kind: agentcli.Read,
				Args:   []agentcli.Arg{{Name: "query", Usage: "case-insensitive text", Required: true}},
				Flags:  []agentcli.Flag{folderFlag, {Name: "body", Type: agentcli.Bool, Usage: "also search note bodies (slower)"}},
				Fields: noteCols, AllFields: noteFields, Run: h.search, Example: "apple-notes search invoice --body --json"},
			{Name: "get", Summary: "show one note as plain text (--html for raw HTML)", Kind: agentcli.Read,
				Args:   []agentcli.Arg{{Name: "id", Usage: "note id", Required: true}},
				Flags:  []agentcli.Flag{{Name: "html", Type: agentcli.Bool, Usage: "include the HTML body"}},
				Fields: []string{"id", "title", "folder", "modified", "locked", "attachments", "text"}, AllFields: noteFields, Run: h.get},
			{Name: "doctor", Summary: "check permissions and connectivity", Kind: agentcli.Read,
				Fields: []string{"check", "ok", "detail"}, Run: h.doctor},
			{Name: "add", Summary: "create notes", Kind: agentcli.Write, Op: "add",
				Args: []agentcli.Arg{{Name: "input", Usage: "'-' to read JSON/NDJSON objects from stdin"}},
				Flags: []agentcli.Flag{
					{Name: "folder", Type: agentcli.String, Usage: "target folder path, name or id"},
					{Name: "title", Type: agentcli.String, Usage: "title (first line)"},
					{Name: "body", Type: agentcli.String, Usage: "body: plain text, or Markdown with --markdown"},
					markdownFlag, attachFlag},
				Stdin:  `{"folder":"iCloud/Agent","title":"…","body":"…","markdown":true,"attach":["/abs/path.png"]} (one per line)`,
				Fields: noteCols, AllFields: noteFields, Plan: h.add,
				Example: "apple-notes add --folder iCloud/Agent --title 'Meeting' --body 'Notes…' --dry-run"},
			{Name: "append", Summary: "append plain text to a note", Kind: agentcli.Write, Op: "append",
				Args:   []agentcli.Arg{{Name: "id", Usage: "note id", Required: true}},
				Flags:  []agentcli.Flag{{Name: "body", Type: agentcli.String, Usage: "text to append", Required: true}, markdownFlag, attachFlag},
				Fields: noteCols, AllFields: noteFields, Plan: h.appendNote},
			{Name: "update", Summary: "replace a note's content (title kept unless --title)", Kind: agentcli.Write, Op: "update",
				Args: []agentcli.Arg{{Name: "id", Usage: "note id", Required: true}},
				Flags: []agentcli.Flag{{Name: "title", Type: agentcli.String, Usage: "new title"},
					{Name: "body", Type: agentcli.String, Usage: "new body: plain text, or Markdown with --markdown", Required: true}, markdownFlag},
				Fields: noteCols, AllFields: noteFields, Plan: h.update},
			{Name: "move", Summary: "move notes to another folder", Kind: agentcli.Write, Op: "move",
				Args:   []agentcli.Arg{{Name: "id", Usage: "note id(s)", Required: true, Variadic: true}},
				Flags:  []agentcli.Flag{{Name: "to", Type: agentcli.String, Usage: "target folder", Required: true}},
				Fields: noteCols, AllFields: noteFields, Plan: h.move},
			{Name: "delete", Summary: "move notes to Recently Deleted", Kind: agentcli.Write, Op: "delete",
				Args:   []agentcli.Arg{{Name: "id", Usage: "note id(s)", Required: true, Variadic: true}},
				Fields: noteCols, AllFields: noteFields, Plan: h.delete},
			{Name: "folder-create", Summary: "create a folder (parent must exist)", Kind: agentcli.Write, Op: "folder-create",
				Args:   []agentcli.Arg{{Name: "path", Usage: "Account/Folder[/Sub]", Required: true}},
				Fields: []string{"id", "path"}, AllFields: folderFields, Plan: h.folderCreate},
			{Name: "folder-delete", Summary: "delete an empty folder", Kind: agentcli.Write, Op: "folder-delete",
				Args:   []agentcli.Arg{{Name: "folder", Usage: "folder path or id", Required: true}},
				Fields: []string{"id", "path"}, AllFields: folderFields, Plan: h.folderDelete},
		},
	}
}

type handlers struct{ b Bridge }

var (
	markdownFlag = agentcli.Flag{Name: "markdown", Type: agentcli.Bool,
		Usage: "body is Markdown: headings, **bold**, *italic*, ~~strike~~, `code`, lists, tables, code blocks, and <u>, <font color>, <span style=\"color|font-size\">"}
	attachFlag = agentcli.Flag{Name: "attach", Type: agentcli.Strings, Usage: "file to attach, e.g. an image (repeatable, max 50 MiB each)"}
)

// maxAttachment caps the size of a file attached to a note.
const maxAttachment = 50 << 20

// attachments validates --attach paths and returns them as absolute paths.
func attachments(paths []string) ([]string, error) {
	var out []string
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, agentcli.Usagef("invalid path %q", p)
		}
		fi, err := os.Stat(abs)
		if err != nil || !fi.Mode().IsRegular() {
			return nil, agentcli.Usagef("%s is not a readable regular file", p)
		}
		if fi.Size() > maxAttachment {
			return nil, agentcli.Usagef("%s is larger than 50 MiB", p)
		}
		out = append(out, abs)
	}
	return out, nil
}

func bodyHTML(title, body string, markdown bool) string {
	if markdown {
		return MarkdownHTML(title, body)
	}
	return NoteHTML(title, body)
}

func folderItem(f Folder) agentcli.Item {
	return agentcli.Item{"id": f.ID, "path": f.Path, "name": f.Name, "account": f.Account, "parent_id": f.ParentID, "notes": f.Notes}
}

func noteItem(n Note) agentcli.Item {
	it := agentcli.Item{"id": n.ID, "title": n.Title, "folder": n.Folder, "folder_id": n.FolderID,
		"created": agentcli.FormatTime(n.Created, false), "modified": agentcli.FormatTime(n.Modified, false),
		"locked": n.Locked, "attachments": n.Attachments}
	if n.Text != "" {
		it["text"] = n.Text
	}
	if n.HTML != "" {
		it["html"] = n.HTML
	}
	return it
}

func (h *handlers) resolveFolder(ctx context.Context, ref string) (Folder, error) {
	fs, err := h.b.Folders(ctx)
	if err != nil {
		return Folder{}, err
	}
	return pickFolder(fs, ref)
}

// pickFolder matches an id, a full path, or a unique folder name.
func pickFolder(fs []Folder, ref string) (Folder, error) {
	var byPath, byName []Folder
	for _, f := range fs {
		switch {
		case f.ID == ref:
			return f, nil
		case f.Path == ref:
			byPath = append(byPath, f)
		case f.Name == ref:
			byName = append(byName, f)
		}
	}
	for _, m := range [][]Folder{byPath, byName} {
		if len(m) == 1 {
			return m[0], nil
		}
		if len(m) > 1 {
			return Folder{}, agentcli.Conflictf("%d folders match %q; use the full path or id", len(m), ref)
		}
	}
	return Folder{}, agentcli.NotFoundf("no folder %q (see: apple-notes folders)", ref)
}

func (h *handlers) accounts(c *agentcli.Ctx) error {
	as, err := h.b.Accounts(c)
	if err != nil {
		return err
	}
	var items []agentcli.Item
	for _, a := range as {
		items = append(items, agentcli.Item{"id": a.ID, "name": a.Name})
	}
	return c.List(items)
}

func (h *handlers) folders(c *agentcli.Ctx) error {
	fs, err := h.b.Folders(c)
	if err != nil {
		return err
	}
	sort.Slice(fs, func(i, j int) bool {
		return fs[i].Path < fs[j].Path || (fs[i].Path == fs[j].Path && fs[i].ID < fs[j].ID)
	})
	var items []agentcli.Item
	for _, f := range fs {
		if c.CanRead(f.Path) && (!c.IsSet("account") || f.Account == c.String("account")) {
			items = append(items, folderItem(f))
		}
	}
	return c.List(items, c.String("account"))
}

// fetch loads notes for --folder flags, filtered by read scope, newest first.
func (h *handlers) fetch(c *agentcli.Ctx, withText bool) ([]Note, error) {
	var ids []string
	if refs := c.Strings("folder"); len(refs) > 0 {
		fs, err := h.b.Folders(c)
		if err != nil {
			return nil, err
		}
		for _, ref := range refs {
			f, err := pickFolder(fs, ref)
			if err != nil {
				return nil, err
			}
			if err := c.CheckRead(f.Path); err != nil {
				return nil, err
			}
			ids = append(ids, f.ID)
		}
	}
	ns, err := h.b.Notes(c, ids, withText)
	if err != nil {
		return nil, err
	}
	out := ns[:0]
	for _, n := range ns {
		if c.CanRead(n.Folder) {
			out = append(out, n)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].Modified.Equal(out[j].Modified) {
			return out[i].Modified.After(out[j].Modified)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (h *handlers) list(c *agentcli.Ctx) error {
	var after, before time.Time
	for flag, dst := range map[string]*time.Time{"modified-after": &after, "modified-before": &before} {
		if s := c.String(flag); s != "" {
			t, _, err := agentcli.ParseTime(s, c.Now(), time.Local)
			if err != nil {
				return err
			}
			*dst = t
		}
	}
	ns, err := h.fetch(c, false)
	if err != nil {
		return err
	}
	var items []agentcli.Item
	for _, n := range ns {
		if (!after.IsZero() && n.Modified.Before(after)) || (!before.IsZero() && !n.Modified.Before(before)) {
			continue
		}
		items = append(items, noteItem(n))
	}
	return c.List(items, strings.Join(c.Strings("folder"), "\x00"), c.String("modified-after"), c.String("modified-before"))
}

func (h *handlers) search(c *agentcli.Ctx) error {
	q := strings.ToLower(c.Args[0])
	ns, err := h.fetch(c, c.Bool("body"))
	if err != nil {
		return err
	}
	var items []agentcli.Item
	for _, n := range ns {
		if strings.Contains(strings.ToLower(n.Title), q) || strings.Contains(strings.ToLower(n.Text), q) {
			n.Text = "" // keep list output compact; use get for content
			items = append(items, noteItem(n))
		}
	}
	return c.List(items, q, strings.Join(c.Strings("folder"), "\x00"), strconv.FormatBool(c.Bool("body")))
}

func (h *handlers) get(c *agentcli.Ctx) error {
	n, err := h.b.Get(c, c.Args[0])
	if err != nil {
		return err
	}
	if err := c.CheckRead(n.Folder); err != nil {
		return err
	}
	if !c.Bool("html") {
		n.HTML = ""
	}
	return c.Show(noteItem(n))
}

func (h *handlers) doctor(c *agentcli.Ctx) error {
	fs, err := h.b.Folders(c)
	row := agentcli.Item{"check": "notes_automation", "ok": err == nil}
	if err != nil {
		row["detail"] = agentcli.AsError(err).Message
	} else {
		row["detail"] = strconv.Itoa(len(fs)) + " folders visible"
	}
	if lerr := c.List([]agentcli.Item{row}); lerr != nil {
		return lerr
	}
	return err
}

type addInput struct {
	Folder   string   `json:"folder"`
	Title    string   `json:"title"`
	Body     string   `json:"body"`
	Markdown bool     `json:"markdown"`
	Attach   []string `json:"attach"`
}

func (h *handlers) add(c *agentcli.Ctx) (*agentcli.WritePlan, error) {
	var ins []addInput
	if len(c.Args) > 0 {
		if c.Args[0] != "-" {
			return nil, agentcli.Usagef("the only positional argument is '-' (stdin)")
		}
		var err error
		if ins, err = agentcli.DecodeInputs[addInput](c.Stdin); err != nil {
			return nil, err
		}
	} else {
		ins = []addInput{{Folder: c.String("folder"), Title: c.String("title"), Body: c.String("body"),
			Markdown: c.Bool("markdown"), Attach: c.Strings("attach")}}
	}
	fs, err := h.b.Folders(c)
	if err != nil {
		return nil, err
	}
	type job struct {
		folder Folder
		html   string
		files  []string
	}
	var jobs []job
	var scopes []string
	var targets []agentcli.Item
	for i, in := range ins {
		if strings.TrimSpace(in.Title) == "" || in.Folder == "" {
			return nil, agentcli.Usagef("item %d: folder and title are required", i+1)
		}
		f, err := pickFolder(fs, in.Folder)
		if err != nil {
			return nil, err
		}
		files, err := attachments(in.Attach)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job{f, bodyHTML(in.Title, in.Body, in.Markdown), files})
		scopes = append(scopes, f.Path)
		targets = append(targets, agentcli.Item{"title": in.Title, "folder": f.Path, "attachments": len(files)})
	}
	return &agentcli.WritePlan{Scopes: scopes, Targets: targets, Apply: func(ctx context.Context) ([]agentcli.Item, error) {
		var out []agentcli.Item
		for _, j := range jobs {
			n, err := h.b.Add(ctx, j.folder.ID, j.html)
			if err != nil {
				return nil, partial(err, len(out), len(jobs))
			}
			for _, f := range j.files {
				if n, err = h.b.Attach(ctx, n.ID, f); err != nil {
					return nil, partial(err, len(out), len(jobs))
				}
			}
			n.Text, n.HTML = "", ""
			out = append(out, noteItem(n))
		}
		return out, nil
	}}, nil
}

// partial annotates an error that happened after some writes succeeded.
func partial(err error, done, total int) error {
	if done == 0 {
		return err
	}
	e := *agentcli.AsError(err)
	e.Message = strconv.Itoa(done) + " of " + strconv.Itoa(total) + " writes succeeded before: " + e.Message
	return &e
}

func (h *handlers) unlocked(c *agentcli.Ctx, id string) (Note, error) {
	n, err := h.b.Get(c, id)
	if err != nil {
		return Note{}, err
	}
	if n.Locked {
		return Note{}, agentcli.Conflictf("note %s is locked (password protected)", id)
	}
	return n, nil
}

func (h *handlers) setBody(n Note, html string, appendBody bool, files []string) *agentcli.WritePlan {
	target := noteItem(Note{ID: n.ID, Title: n.Title, Folder: n.Folder, FolderID: n.FolderID, Modified: n.Modified})
	return &agentcli.WritePlan{Scopes: []string{n.Folder}, Targets: []agentcli.Item{target},
		Apply: func(ctx context.Context) ([]agentcli.Item, error) {
			u, err := h.b.SetBody(ctx, n.ID, html, appendBody)
			if err != nil {
				return nil, err
			}
			for _, f := range files {
				if u, err = h.b.Attach(ctx, n.ID, f); err != nil {
					return nil, err
				}
			}
			u.Text, u.HTML = "", ""
			return []agentcli.Item{noteItem(u)}, nil
		}}
}

func (h *handlers) appendNote(c *agentcli.Ctx) (*agentcli.WritePlan, error) {
	n, err := h.unlocked(c, c.Args[0])
	if err != nil {
		return nil, err
	}
	if c.String("body") == "" {
		return nil, agentcli.Usagef("--body must not be empty")
	}
	files, err := attachments(c.Strings("attach"))
	if err != nil {
		return nil, err
	}
	return h.setBody(n, bodyHTML("", c.String("body"), c.Bool("markdown")), true, files), nil
}

func (h *handlers) update(c *agentcli.Ctx) (*agentcli.WritePlan, error) {
	n, err := h.unlocked(c, c.Args[0])
	if err != nil {
		return nil, err
	}
	title := n.Title
	if c.IsSet("title") {
		if title = c.String("title"); strings.TrimSpace(title) == "" {
			return nil, agentcli.Usagef("title must not be empty")
		}
	}
	p := h.setBody(n, bodyHTML(title, c.String("body"), c.Bool("markdown")), false, nil)
	p.Targets[0]["title"] = title
	return p, nil
}

// each loads every id and builds a plan applying fn; extra scopes are added.
func (h *handlers) each(c *agentcli.Ctx, extra []string, fn func(context.Context, Note) (agentcli.Item, error)) (*agentcli.WritePlan, error) {
	var ns []Note
	scopes := append([]string{}, extra...)
	var targets []agentcli.Item
	for _, id := range c.Args {
		n, err := h.b.Get(c, id)
		if err != nil {
			return nil, err
		}
		n.Text, n.HTML = "", ""
		ns = append(ns, n)
		scopes = append(scopes, n.Folder)
		targets = append(targets, noteItem(n))
	}
	return &agentcli.WritePlan{Scopes: scopes, Targets: targets, Apply: func(ctx context.Context) ([]agentcli.Item, error) {
		var out []agentcli.Item
		for _, n := range ns {
			it, err := fn(ctx, n)
			if err != nil {
				return nil, partial(err, len(out), len(ns))
			}
			out = append(out, it)
		}
		return out, nil
	}}, nil
}

func (h *handlers) move(c *agentcli.Ctx) (*agentcli.WritePlan, error) {
	to, err := h.resolveFolder(c, c.String("to"))
	if err != nil {
		return nil, err
	}
	p, err := h.each(c, []string{to.Path}, func(ctx context.Context, n Note) (agentcli.Item, error) {
		u, err := h.b.Move(ctx, n.ID, to.ID)
		if err != nil {
			return nil, err
		}
		return noteItem(u), nil
	})
	if err != nil {
		return nil, err
	}
	for _, t := range p.Targets {
		t["to"] = to.Path
	}
	return p, nil
}

func (h *handlers) delete(c *agentcli.Ctx) (*agentcli.WritePlan, error) {
	return h.each(c, nil, func(ctx context.Context, n Note) (agentcli.Item, error) {
		if err := h.b.Delete(ctx, n.ID, false); err != nil {
			return nil, err
		}
		it := noteItem(n)
		it["deleted"] = true
		return it, nil
	})
}

func (h *handlers) folderCreate(c *agentcli.Ctx) (*agentcli.WritePlan, error) {
	path := strings.Trim(c.Args[0], "/ ")
	parentPath, name, ok := cutLast(path)
	if !ok || strings.TrimSpace(name) == "" {
		return nil, agentcli.Usagef("path must be Account/Folder[/Sub]")
	}
	fs, err := h.b.Folders(c)
	if err != nil {
		return nil, err
	}
	for _, f := range fs {
		if f.Path == path {
			return nil, agentcli.Conflictf("folder %q already exists", path)
		}
	}
	var accountID, parentID string
	if !strings.Contains(parentPath, "/") {
		as, err := h.b.Accounts(c)
		if err != nil {
			return nil, err
		}
		for _, a := range as {
			if a.Name == parentPath {
				accountID = a.ID
			}
		}
		if accountID == "" {
			return nil, agentcli.NotFoundf("no account %q", parentPath)
		}
	} else {
		parent, err := pickFolder(fs, parentPath)
		if err != nil {
			return nil, err
		}
		parentID = parent.ID
	}
	return &agentcli.WritePlan{Scopes: []string{path}, Targets: []agentcli.Item{{"path": path}},
		Apply: func(ctx context.Context) ([]agentcli.Item, error) {
			f, err := h.b.CreateFolder(ctx, accountID, parentID, name)
			if err != nil {
				return nil, err
			}
			return []agentcli.Item{folderItem(f)}, nil
		}}, nil
}

func cutLast(p string) (string, string, bool) {
	i := strings.LastIndex(p, "/")
	if i <= 0 {
		return "", "", false
	}
	return p[:i], p[i+1:], true
}

func (h *handlers) folderDelete(c *agentcli.Ctx) (*agentcli.WritePlan, error) {
	f, err := h.resolveFolder(c, c.Args[0])
	if err != nil {
		return nil, err
	}
	return &agentcli.WritePlan{Scopes: []string{f.Path}, Targets: []agentcli.Item{folderItem(f)},
		Apply: func(ctx context.Context) ([]agentcli.Item, error) {
			if err := h.b.DeleteFolder(ctx, f.ID, false); err != nil {
				return nil, err
			}
			it := folderItem(f)
			it["deleted"] = true
			return []agentcli.Item{it}, nil
		}}, nil
}
