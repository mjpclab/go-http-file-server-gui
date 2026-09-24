package main

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	. "modernc.org/tk9.0"
)

// maxLogLines caps the Logs text so a long-running server cannot grow it
// without bound; the oldest lines are dropped first.
const maxLogLines = 10000

const errorLogTag = "error"

// logErrorColor is set by applySystemTheme, before the tab is built.
var logErrorColor = "#c62828"

// logsTab is the Logs tab: one read-only text receiving the access and error
// logs of the running server, interleaved in arrival order.
type logsTab struct {
	frame *TFrameWidget
	text  *TextWidget

	// The writers handed to ghfs. They must be distinct values: ghfs's
	// WriterMan shares one queue between loggers given the same writer.
	accessLog *logWriter
	errorLog  *logWriter

	mu      sync.Mutex
	pending []logEntry
	posted  bool
}

type logEntry struct {
	isError bool
	text    string
}

// logWriter receives one complete log line per Write, from a ghfs goroutine.
type logWriter struct {
	tab     *logsTab
	isError bool
}

func (w *logWriter) Write(p []byte) (int, error) {
	text := strings.TrimRight(string(p), "\r\n")
	w.tab.enqueue(logEntry{isError: w.isError, text: text})
	return len(p), nil
}

func newLogsTab(parent *Window) *logsTab {
	l := &logsTab{}
	l.accessLog = &logWriter{tab: l}
	l.errorLog = &logWriter{tab: l, isError: true}

	l.frame = parent.TFrame(Padding("2m"))
	sb := l.frame.TScrollbar()
	l.text = l.frame.Text(
		Wrap("char"),
		Font("TkFixedFont"),
		State("disabled"),
		Highlightthickness(0),
		Yscrollcommand(func(e *Event) { e.ScrollSet(sb) }),
	)
	sb.Configure(Command(func(e *Event) { e.Yview(l.text) }))
	l.text.TagConfigure(errorLogTag, Foreground(logErrorColor))

	Grid(l.text, Row(0), Column(0), Sticky("news"))
	Grid(sb, Row(0), Column(1), Sticky("ns"))
	GridRowConfigure(l.frame, 0, Weight(1))
	GridColumnConfigure(l.frame, 0, Weight(1))
	return l
}

// enqueue batches lines until the GUI thread picks them up. At most one flush
// is in flight: PostEvent blocks when Tk's event queue is full, and Stop waits
// on the GUI thread for ghfs to drain its log queues into these writers, so a
// post per line could deadlock under heavy traffic.
func (l *logsTab) enqueue(e logEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pending = append(l.pending, e)
	if !l.posted {
		l.posted = true
		PostEvent(l.flush, false)
	}
}

func (l *logsTab) flush() {
	l.mu.Lock()
	entries := l.pending
	l.pending = nil
	l.posted = false
	l.mu.Unlock()

	// Follow the tail only while the user is at it, so scrolling back to read
	// an earlier line is not undone by the next request.
	follow := l.atBottom()

	l.text.Configure(State("normal"))
	// Lines are separated rather than terminated, or the view would always end
	// on an empty line.
	empty := l.text.Index("end-1c") == "1.0"
	for _, e := range entries {
		text := e.text
		if !empty {
			text = "\n" + text
		}
		empty = false
		if e.isError {
			l.text.Insert("end", text, errorLogTag)
		} else {
			l.text.Insert("end", text)
		}
	}
	l.trim()
	l.text.Configure(State("disabled"))

	if follow {
		l.text.See("end")
	}
}

func (l *logsTab) atBottom() bool {
	fields := strings.Fields(l.text.Yview())
	if len(fields) != 2 {
		return true
	}
	end, err := strconv.ParseFloat(fields[1], 64)
	return err != nil || end >= 1
}

// trim drops the oldest lines beyond maxLogLines. Tk keeps a newline of its own
// after the last line, so "end" is one line past it.
func (l *logsTab) trim() {
	line, _, _ := strings.Cut(l.text.Index("end"), ".")
	n, err := strconv.Atoi(line)
	if err != nil {
		return
	}
	if extra := n - 1 - maxLogLines; extra > 0 {
		l.text.Delete("1.0", fmt.Sprintf("%d.0", extra+1))
	}
}
