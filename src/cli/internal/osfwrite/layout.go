// Package osfwrite renders the project model as .osf source in the canonical
// layout of language.md §11, already formatted the way osf fmt leaves it.
package osfwrite

import (
	"bytes"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/format"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"
)

// File is one rendered .osf file. Path is relative to the output folder, with
// / separators.
type File struct {
	Path    string
	Content []byte
}

// Project renders p as .osf files. origin returns, for a resource's source
// file, the .osf file it was generated from (see yamlio.GeneratedFrom), or ""
// to use the default layout: versions.osf, story.osf (Story and I18n),
// characters.osf, documents.osf, variables.osf, <folder>/<dialogue id>.osf, and
// locales/<Lang>/<source path>.osf for translations.
func Project(p *osf.Project, origin func(file string) string) ([]File, error) {
	if origin == nil {
		origin = func(string) string { return "" }
	}
	target := func(file, dflt string) string {
		if o := origin(file); o != "" {
			return path.Clean(o)
		}
		return dflt
	}
	files := map[string][]block{}
	add := func(file string, b block) { files[file] = append(files[file], b) }

	add("versions.osf", versions(p))
	for _, s := range p.Stories {
		add(target(s.File, "story.osf"), render(kindStory, func(w *writer) { story(w, s) }))
	}
	for _, c := range p.I18n {
		add(target(c.File, "story.osf"), render(kindI18n, func(w *writer) { i18n(w, c) }))
	}
	for _, c := range p.Characters {
		add(target(c.File, "characters.osf"), render(kindCharacter, func(w *writer) { character(w, c) }))
	}
	for _, d := range p.Documents {
		add(target(d.File, "documents.osf"), render(kindDocument, func(w *writer) { document(w, d) }))
	}
	for _, v := range p.Variables {
		add(target(v.File, "variables.osf"), render(kindVariable, func(w *writer) { variable(w, v) }))
	}
	for _, d := range p.Dialogues {
		add(target(d.File, path.Join(path.Dir(d.File), d.ID+".osf")), render(kindDialogue, func(w *writer) { dialogue(w, d, false) }))
	}
	// Locale resources of one language that land in the same file become one
	// block, as osf convert made one resource per Locale file.
	type key struct{ file, lang string }
	merged := map[key]*osf.Locale{}
	var order []key
	for _, l := range p.Locales {
		k := key{target(l.File, localePath(l)), l.Language.Value}
		if merged[k] == nil {
			merged[k] = &osf.Locale{Language: l.Language, Comments: l.Comments}
			order = append(order, k)
		}
		merged[k].Values = append(merged[k].Values, l.Values...)
	}
	for _, k := range order {
		b, err := locale(merged[k])
		if err != nil {
			return nil, err
		}
		add(k.file, b)
	}

	var out []File
	for file, blocks := range files {
		content, err := join(file, blocks)
		if err != nil {
			return nil, err
		}
		out = append(out, File{Path: file, Content: content})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// Source renders every resource of p as one .osf file in §11 order. The
// Versions block is written only when withVersions is set, since a single
// converted resource has no versions of its own.
func Source(p *osf.Project, withVersions bool) ([]byte, error) {
	var blocks []block
	if withVersions {
		blocks = append(blocks, versions(p))
	}
	for _, s := range p.Stories {
		blocks = append(blocks, render(kindStory, func(w *writer) { story(w, s) }))
	}
	for _, c := range p.I18n {
		blocks = append(blocks, render(kindI18n, func(w *writer) { i18n(w, c) }))
	}
	for _, c := range p.Characters {
		blocks = append(blocks, render(kindCharacter, func(w *writer) { character(w, c) }))
	}
	for _, d := range p.Documents {
		blocks = append(blocks, render(kindDocument, func(w *writer) { document(w, d) }))
	}
	for _, v := range p.Variables {
		blocks = append(blocks, render(kindVariable, func(w *writer) { variable(w, v) }))
	}
	for _, d := range p.Dialogues {
		blocks = append(blocks, render(kindDialogue, func(w *writer) { dialogue(w, d, false) }))
	}
	merged := map[string]*osf.Locale{}
	var order []string
	for _, l := range p.Locales {
		lang := l.Language.Value
		if merged[lang] == nil {
			merged[lang] = &osf.Locale{Language: l.Language, Comments: l.Comments}
			order = append(order, lang)
		}
		merged[lang].Values = append(merged[lang].Values, l.Values...)
	}
	for _, lang := range order {
		b, err := locale(merged[lang])
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, b)
	}
	return join("converted.osf", blocks)
}

// localePath is where a Locale goes by default: locales/<Lang>/ plus the
// source file's path, kept as is when it already sits in that folder.
func localePath(l *osf.Locale) string {
	lang := l.Language.Value
	rel := strings.TrimSuffix(l.File, path.Ext(l.File)) + ".osf"
	if folder := localeFolder(rel); folder != "" {
		if folder == lang {
			return rel
		}
		rel = strings.TrimPrefix(rel, "locales/"+folder+"/")
	}
	return path.Join("locales", lang, rel)
}

// localeFolder returns X when rel is inside locales/X/, else "".
func localeFolder(rel string) string {
	parts := strings.Split(rel, "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] == "locales" {
			return parts[i+1]
		}
	}
	return ""
}

// join writes blocks in §11 order, one blank line between them except
// between consecutive Variable lines. The result goes through osf fmt, which
// settles what the layout rules leave open (the alignment of trailing
// comments) and guards against output that would not parse.
func join(file string, blocks []block) ([]byte, error) {
	sort.SliceStable(blocks, func(i, j int) bool { return blocks[i].kind < blocks[j].kind })
	var b bytes.Buffer
	for i, bl := range blocks {
		if i > 0 && !(bl.kind == kindVariable && blocks[i-1].kind == kindVariable) {
			b.WriteByte('\n')
		}
		for _, l := range bl.lines {
			b.WriteString(l)
			b.WriteByte('\n')
		}
	}
	out, err := format.Source(file, b.Bytes())
	if err != nil {
		return nil, fmt.Errorf("%s: the written source does not parse (please report this): %w\n%s", file, err, b.Bytes())
	}
	return out, nil
}
