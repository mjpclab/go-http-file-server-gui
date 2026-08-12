package main

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// perm is one of the four directory-level permissions ghfs can grant.
type perm uint8

const (
	permArchive perm = 1 << iota
	permUpload
	permMkdir
	permDelete
)

// permOrder fixes the order used by the Directory tab's parallel arrays and by
// the tree's abbreviation column. It matches the General tab's layout.
var permOrder = [4]perm{permArchive, permUpload, permMkdir, permDelete}

var permLabels = map[perm]string{
	permArchive: "Archive",
	permUpload:  "Upload",
	permMkdir:   "Mkdir",
	permDelete:  "Delete",
}

// permKeys are the stable names written to preference.json.
var permKeys = map[perm]string{
	permArchive: "archive",
	permUpload:  "upload",
	permMkdir:   "mkdir",
	permDelete:  "delete",
}

var permAbbrs = map[perm]string{
	permArchive: "A",
	permUpload:  "U",
	permMkdir:   "M",
	permDelete:  "D",
}

// caseInsensitiveFS mirrors ghfs, which compares filesystem paths
// case-insensitively on windows and darwin (src/util/path_case_insensitive.go).
var caseInsensitiveFS = runtime.GOOS == "windows" || runtime.GOOS == "darwin"

func pathEqual(a, b string) bool {
	if caseInsensitiveFS {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// hasPathPrefix reports whether p is prefix or lies below it, using the same
// separator-boundary rule as ghfs's util.hasPrefixDirAccurate. Keeping this in
// step with ghfs matters: it is what makes the UI's inheritance display agree
// with the permissions the server actually applies.
func hasPathPrefix(p, prefix string) bool {
	if pathEqual(p, prefix) {
		return true
	}
	if len(p) <= len(prefix) {
		return false
	}
	if !pathEqual(p[:len(prefix)], prefix) {
		return false
	}
	if prefix[len(prefix)-1] == filepath.Separator {
		return true
	}
	return p[len(prefix)] == filepath.Separator
}

// nativePath rewrites a path with the platform's separator. Tk's file dialogs
// hand back Tcl-style paths — "D:/Downloads" on Windows — while everything on
// the Go side (dirPerms keys, the Directory tree, hasPathPrefix's separator
// test) goes through filepath.Clean, so entry text is normalized on the way in
// to keep the two spellings from diverging on screen and in comparisons.
func nativePath(p string) string {
	if p == "" {
		return "" // filepath.Clean("") is ".", which is not an empty entry.
	}
	return filepath.Clean(p)
}

// dirPerms maps a cleaned absolute directory path to the permissions explicitly
// granted on it. ghfs grants by path prefix, so an entry also covers every
// descendant of that directory; there is no way to revoke an inherited grant.
type dirPerms struct{ m map[string]perm }

func newDirPerms() *dirPerms {
	return &dirPerms{m: map[string]perm{}}
}

// get returns the bits explicitly granted on dir itself.
func (p *dirPerms) get(dir string) perm {
	return p.m[filepath.Clean(dir)]
}

func (p *dirPerms) set(dir string, bit perm, on bool) {
	dir = filepath.Clean(dir)
	v := p.m[dir]
	if on {
		v |= bit
	} else {
		v &^= bit
	}
	if v == 0 {
		delete(p.m, dir)
		return
	}
	p.m[dir] = v
}

// inherited returns the bits dir receives from an ancestor entry, along with
// the nearest granting ancestor for each bit so the UI can name the source.
func (p *dirPerms) inherited(dir string) (bits perm, from map[perm]string) {
	dir = filepath.Clean(dir)
	from = map[perm]string{}
	for anc, v := range p.m {
		if pathEqual(anc, dir) || !hasPathPrefix(dir, anc) {
			continue
		}
		for _, bit := range permOrder {
			if v&bit == 0 {
				continue
			}
			bits |= bit
			if cur, ok := from[bit]; !ok || len(anc) > len(cur) {
				from[bit] = anc
			}
		}
	}
	return
}

// dirsWith returns the directories carrying bit, for one of param.Param's
// ArchiveDirs/UploadDirs/MkdirDirs/DeleteDirs fields.
func (p *dirPerms) dirsWith(bit perm) []string {
	var dirs []string
	for dir, v := range p.m {
		if v&bit != 0 {
			dirs = append(dirs, dir)
		}
	}
	sort.Strings(dirs)
	return dirs
}

// abbr renders dir's permissions for the tree's second column: uppercase for a
// grant made on dir itself, lowercase for one inherited from an ancestor.
// Globally granted permissions are deliberately left out — with a global switch
// on, every row would carry the same letter and the column would stop informing.
func (p *dirPerms) abbr(dir string) string {
	own := p.get(dir)
	inherited, _ := p.inherited(dir)

	var b strings.Builder
	for _, bit := range permOrder {
		var s string
		switch {
		case own&bit != 0:
			s = permAbbrs[bit]
		case inherited&bit != 0:
			s = strings.ToLower(permAbbrs[bit])
		default:
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(s)
	}
	return b.String()
}

// prune drops entries outside root or no longer present on disk. Grants name
// absolute filesystem paths, so they stop meaning anything once root moves —
// re-applying them to a same-named directory elsewhere would silently expose a
// different set of files.
func (p *dirPerms) prune(root string) {
	if root == "" {
		p.m = map[string]perm{}
		return
	}
	root = filepath.Clean(root)
	for dir := range p.m {
		if !hasPathPrefix(dir, root) {
			delete(p.m, dir)
			continue
		}
		if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
			delete(p.m, dir)
		}
	}
}

func (p *dirPerms) toJSON() map[string][]string {
	if len(p.m) == 0 {
		return nil
	}
	out := make(map[string][]string, len(p.m))
	for dir, v := range p.m {
		var keys []string
		for _, bit := range permOrder {
			if v&bit != 0 {
				keys = append(keys, permKeys[bit])
			}
		}
		out[dir] = keys
	}
	return out
}

func dirPermsFromJSON(m map[string][]string) *dirPerms {
	p := newDirPerms()
	for dir, keys := range m {
		var v perm
		for _, key := range keys {
			for _, bit := range permOrder {
				if permKeys[bit] == key {
					v |= bit
				}
			}
		}
		if v != 0 {
			p.m[filepath.Clean(dir)] = v
		}
	}
	return p
}
