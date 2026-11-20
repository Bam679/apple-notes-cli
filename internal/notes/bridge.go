// Package notes implements the apple-notes CLI on top of a Bridge to
// Notes.app (JXA in production, a fake in tests).
package notes

import (
	"context"
	_ "embed"
	"html"
	"strings"
	"time"

	"github.com/Bam679/apple-notes-cli/internal/osa"
)

// Account is a Notes account (iCloud, On My Mac, IMAP…).
type Account struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Folder is a Notes folder; Path is "Account/Parent/Name".
type Folder struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Account  string `json:"account"`
	ParentID string `json:"parent_id"`
	Notes    int    `json:"notes"`
}

// Note is a note. Text and HTML are only filled by Get (and Text by
// Notes with text when searching bodies).
type Note struct {
	ID          string
	Title       string
	Folder      string
	FolderID    string
	Created     time.Time
	Modified    time.Time
	Locked      bool
	Attachments int
	Text        string
	HTML        string
}

// Bridge is the OS boundary. Implementations must not interpret input as code.
type Bridge interface {
	Accounts(ctx context.Context) ([]Account, error)
	Folders(ctx context.Context) ([]Folder, error)
	Notes(ctx context.Context, folderIDs []string, withText bool) ([]Note, error)
	Get(ctx context.Context, id string) (Note, error)
	Add(ctx context.Context, folderID, html string) (Note, error)
	SetBody(ctx context.Context, id, html string, appendBody bool) (Note, error)
	Move(ctx context.Context, id, folderID string) (Note, error)
	// Delete moves a note to "Recently Deleted"; purge (test cleanup only)
	// deletes it from there as well.
	Delete(ctx context.Context, id string, purge bool) error
	CreateFolder(ctx context.Context, accountID, parentID, name string) (Folder, error)
	// DeleteFolder refuses non-empty folders unless force (test cleanup only).
	DeleteFolder(ctx context.Context, id string, force bool) error
}

// NoteHTML renders a title and plain-text body as Notes HTML. All text is
// HTML-escaped; the result is data, never script.
func NoteHTML(title, body string) string {
	var b strings.Builder
	if title != "" {
		b.WriteString("<div><h1>" + html.EscapeString(title) + "</h1></div>")
	}
	b.WriteString(BodyHTML(body))
	return b.String()
}

// BodyHTML renders plain text as Notes HTML paragraphs.
func BodyHTML(body string) string {
	if body == "" {
		return ""
	}
	var b strings.Builder
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		if line == "" {
			b.WriteString("<div><br></div>")
			continue
		}
		b.WriteString("<div>" + html.EscapeString(line) + "</div>")
	}
	return b.String()
}

//go:embed notes.js
var script string

// JXA is the production Bridge backed by Notes.app through osascript.
type JXA struct {
	Runner osa.Runner
}

// NewJXA returns the production bridge.
func NewJXA() *JXA { return &JXA{Runner: osa.Exec{}} }

type wireNote struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Folder      string `json:"folder"`
	FolderID    string `json:"folder_id"`
	CreatedMS   *int64 `json:"created_ms"`
	ModifiedMS  *int64 `json:"modified_ms"`
	Locked      bool   `json:"locked"`
	Attachments int    `json:"attachments"`
	Text        string `json:"text"`
	HTML        string `json:"html"`
}

func fromMS(v *int64) time.Time {
	if v == nil {
		return time.Time{}
	}
	return time.UnixMilli(*v).Local()
}

func (w wireNote) note() Note {
	return Note{ID: w.ID, Title: w.Title, Folder: w.Folder, FolderID: w.FolderID, Created: fromMS(w.CreatedMS),
		Modified: fromMS(w.ModifiedMS), Locked: w.Locked, Attachments: w.Attachments, Text: w.Text, HTML: w.HTML}
}

func (j *JXA) one(ctx context.Context, op string, args any) (Note, error) {
	var w wireNote
	if err := j.Runner.Run(ctx, script, op, args, &w); err != nil {
		return Note{}, err
	}
	return w.note(), nil
}

// Accounts implements Bridge.
func (j *JXA) Accounts(ctx context.Context) ([]Account, error) {
	var out []Account
	err := j.Runner.Run(ctx, script, "accounts", nil, &out)
	return out, err
}

// Folders implements Bridge.
func (j *JXA) Folders(ctx context.Context) ([]Folder, error) {
	var out []Folder
	err := j.Runner.Run(ctx, script, "folders", nil, &out)
	return out, err
}

// Notes implements Bridge.
func (j *JXA) Notes(ctx context.Context, folderIDs []string, withText bool) ([]Note, error) {
	var ws []wireNote
	if err := j.Runner.Run(ctx, script, "notes", map[string]any{"folder_ids": folderIDs, "with_text": withText}, &ws); err != nil {
		return nil, err
	}
	out := make([]Note, len(ws))
	for i, w := range ws {
		out[i] = w.note()
	}
	return out, nil
}

// Get implements Bridge.
func (j *JXA) Get(ctx context.Context, id string) (Note, error) {
	return j.one(ctx, "get", map[string]string{"id": id})
}

// Add implements Bridge.
func (j *JXA) Add(ctx context.Context, folderID, html string) (Note, error) {
	return j.one(ctx, "add", map[string]string{"folder_id": folderID, "html": html})
}

// SetBody implements Bridge.
func (j *JXA) SetBody(ctx context.Context, id, html string, appendBody bool) (Note, error) {
	return j.one(ctx, "set_body", map[string]any{"id": id, "html": html, "append": appendBody})
}

// Move implements Bridge.
func (j *JXA) Move(ctx context.Context, id, folderID string) (Note, error) {
	return j.one(ctx, "move", map[string]string{"id": id, "folder_id": folderID})
}

// Delete implements Bridge.
func (j *JXA) Delete(ctx context.Context, id string, purge bool) error {
	return j.Runner.Run(ctx, script, "delete", map[string]any{"id": id, "purge": purge}, nil)
}

// CreateFolder implements Bridge.
func (j *JXA) CreateFolder(ctx context.Context, accountID, parentID, name string) (Folder, error) {
	var f Folder
	err := j.Runner.Run(ctx, script, "folder_create", map[string]string{"account_id": accountID, "parent_id": parentID, "name": name}, &f)
	return f, err
}

// DeleteFolder implements Bridge.
func (j *JXA) DeleteFolder(ctx context.Context, id string, force bool) error {
	return j.Runner.Run(ctx, script, "folder_delete", map[string]any{"id": id, "force": force}, nil)
}
