package notes

import (
	"html"
	"regexp"
	"strconv"
	"strings"
)

// MarkdownHTML converts Markdown into the HTML subset that Notes keeps when a
// note is written through scripting: headings, bold, italic, underline,
// strikethrough, monospace, colours, font sizes, bullet and numbered lists,
// tables and code blocks. Notes drops checklists, highlights, block quotes,
// alignment and link targets, so they are rendered as plain text.
//
// Inline HTML is limited to <u>, <b>, <i>, <s>, <font color|face> and
// <span style="color|font-size|font-family">; every other tag is escaped.
func MarkdownHTML(title, md string) string {
	var b strings.Builder
	if title != "" {
		b.WriteString(`<div><b><span style="font-size: 24px">` + html.EscapeString(title) + `</span></b></div>`)
	}
	lines := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	var list []string // open list tags, innermost last
	closeLists := func(depth int) {
		for len(list) > depth {
			b.WriteString("</" + list[len(list)-1] + ">")
			list = list[:len(list)-1]
		}
	}
	lastList := ""
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "```"):
			closeLists(0)
			for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```"); i++ {
				b.WriteString(`<div><font face="Courier">` + nbsp(html.EscapeString(lines[i])) + `</font></div>`)
			}
		case isTableRow(trimmed) && i+1 < len(lines) && isTableRule(strings.TrimSpace(lines[i+1])):
			closeLists(0)
			b.WriteString("<table><tbody>")
			b.WriteString(tableRow(trimmed, true))
			for i += 2; i < len(lines) && isTableRow(strings.TrimSpace(lines[i])); i++ {
				b.WriteString(tableRow(strings.TrimSpace(lines[i]), false))
			}
			i--
			b.WriteString("</tbody></table>")
		case listItem.MatchString(line):
			m := listItem.FindStringSubmatch(line)
			depth := len(strings.ReplaceAll(m[1], "\t", "  "))/2 + 1
			kind := "ul"
			if m[2] != "-" && m[2] != "*" && m[2] != "+" {
				kind = "ol"
			}
			if len(list) == 0 && lastList != "" && lastList != kind {
				// Notes merges adjacent lists of different kinds without a gap.
				b.WriteString("<div><br></div>")
			}
			closeLists(depth)
			if len(list) == depth && list[depth-1] != kind {
				closeLists(depth - 1)
				if depth == 1 {
					b.WriteString("<div><br></div>") // keep Notes from merging the lists
				}
			}
			for len(list) < depth {
				b.WriteString("<" + kind + ">")
				list = append(list, kind)
			}
			b.WriteString("<li>" + inline(m[3]) + "</li>")
			lastList = kind
		default:
			closeLists(0)
			if trimmed != "" {
				lastList = ""
			}
			switch {
			case trimmed == "":
				b.WriteString("<div><br></div>")
			case strings.HasPrefix(trimmed, "### "):
				b.WriteString("<div><b>" + inline(trimmed[4:]) + "</b></div>")
			case strings.HasPrefix(trimmed, "## "):
				b.WriteString(`<div><b><span style="font-size: 18px">` + inline(trimmed[3:]) + `</span></b></div>`)
			case strings.HasPrefix(trimmed, "# "):
				b.WriteString(`<div><b><span style="font-size: 24px">` + inline(trimmed[2:]) + `</span></b></div>`)
			case strings.HasPrefix(trimmed, "> "):
				b.WriteString(`<div><i>` + inline(trimmed[2:]) + `</i></div>`)
			case hrule.MatchString(trimmed):
				b.WriteString("<div><br></div>")
			default:
				b.WriteString("<div>" + inline(line) + "</div>")
			}
		}
	}
	closeLists(0)
	return b.String()
}

var (
	listItem   = regexp.MustCompile(`^(\s*)([-*+]|\d{1,9}[.)])\s+(.*)$`)
	hrule      = regexp.MustCompile(`^(-{3,}|\*{3,}|_{3,})$`)
	tableRule  = regexp.MustCompile(`^\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)*\|?$`)
	inlineCode = regexp.MustCompile("`([^`]+)`")
	boldRe     = regexp.MustCompile(`\*\*([^*]+)\*\*|__([^_]+)__`)
	italicRe   = regexp.MustCompile(`\*([^*]+)\*|\b_([^_]+)_\b`)
	strikeRe   = regexp.MustCompile(`~~([^~]+)~~`)
	tagRe      = regexp.MustCompile(`(?i)&lt;(/?)(u|b|i|s|font|span)((?:\s+[a-z-]+=&#34;[^&]*&#34;)*)\s*&gt;`)
	attrRe     = regexp.MustCompile(`(?i)([a-z-]+)=&#34;([^&]*)&#34;`)
	styleOK    = regexp.MustCompile(`^\s*(color|font-size|font-family)\s*:\s*[#a-zA-Z0-9 ,.()%-]+\s*$`)
	valueOK    = regexp.MustCompile(`^[#a-zA-Z0-9 ,.()%-]+$`)
)

func isTableRow(s string) bool {
	return strings.HasPrefix(s, "|") && strings.HasSuffix(s, "|") && len(s) > 1
}
func isTableRule(s string) bool { return tableRule.MatchString(s) }

func tableRow(s string, header bool) string {
	cells := strings.Split(strings.Trim(s, "|"), "|")
	var b strings.Builder
	b.WriteString("<tr>")
	for _, c := range cells {
		v := inline(strings.TrimSpace(c))
		if header {
			v = "<b>" + v + "</b>"
		}
		b.WriteString("<td>" + v + "</td>")
	}
	b.WriteString("</tr>")
	return b.String()
}

func nbsp(s string) string {
	trimmed := strings.TrimLeft(s, " ")
	return strings.Repeat("&nbsp;", len(s)-len(trimmed)) + trimmed
}

// inline escapes text, then re-enables the allowed inline tags and applies
// Markdown emphasis. Code spans are protected from further formatting.
func inline(s string) string {
	esc := html.EscapeString(s)
	var codes []string
	esc = inlineCode.ReplaceAllStringFunc(esc, func(m string) string {
		codes = append(codes, `<font face="Courier">`+m[1:len(m)-1]+`</font>`)
		return "\x00" + strconv.Itoa(len(codes)-1) + "\x00"
	})
	esc = tagRe.ReplaceAllStringFunc(esc, allowTag)
	esc = boldRe.ReplaceAllString(esc, "<b>$1$2</b>")
	esc = strikeRe.ReplaceAllString(esc, "<strike>$1</strike>")
	esc = italicRe.ReplaceAllString(esc, "<i>$1$2</i>")
	for i, c := range codes {
		esc = strings.Replace(esc, "\x00"+strconv.Itoa(i)+"\x00", c, 1)
	}
	return esc
}

func allowTag(m string) string {
	p := tagRe.FindStringSubmatch(m)
	closing, name, attrs := p[1] == "/", strings.ToLower(p[2]), p[3]
	if closing {
		return "</" + name + ">"
	}
	var kept []string
	for _, a := range attrRe.FindAllStringSubmatch(attrs, -1) {
		key, val := strings.ToLower(a[1]), html.UnescapeString(a[2])
		switch {
		case name == "font" && (key == "color" || key == "face") && valueOK.MatchString(val):
		case name == "span" && key == "style" && styleOK.MatchString(val):
		default:
			continue
		}
		kept = append(kept, key+`="`+html.EscapeString(val)+`"`)
	}
	if (name == "font" || name == "span") && len(kept) == 0 {
		return m // nothing safe left: keep it escaped
	}
	if len(kept) > 0 {
		return "<" + name + " " + strings.Join(kept, " ") + ">"
	}
	return "<" + name + ">"
}
