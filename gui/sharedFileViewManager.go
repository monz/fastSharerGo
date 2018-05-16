package ui

import (
	"fmt"
	"github.com/jroimartin/gocui"
	commonData "github.com/monz/fastSharerGo/common/data"
	"log"
	"sync"
)

type fileInfo struct {
	name            string
	chunkCount      int
	toDownloadCount int
}

type SharedFileViewMgr struct {
	ViewManager // inherit ViewManager
	sharedFiles map[string]*commonData.SharedFile
	mu          sync.Mutex
}

func NewSharedFileViewMgr(name, title string, editable, wrap, autoscroll bool, x0, y0, x1, y1 int) *SharedFileViewMgr {
	m := new(SharedFileViewMgr)
	m.name = name
	m.title = title
	m.editable = editable
	m.wrap = wrap
	m.autoscroll = autoscroll
	m.x0 = x0
	m.y0 = y0
	m.x1 = x1
	m.y1 = y1
	m.sharedFiles = make(map[string]*commonData.SharedFile)

	return m
}

func (m *SharedFileViewMgr) Update(g *gocui.Gui, sf *commonData.SharedFile) {
	// create ui
	if err := m.Layout(g); err != nil {
		log.Fatal(err)
	}
	// update ui
	sfs := make([]fileInfo, 0, len(m.sharedFiles))
	for _, sf := range m.sharedFiles {
		sfs = append(sfs, fileInfo{sf.FileName(), len(sf.Chunks()), len(sf.ChunksToDownload())})
	}
	g.Update(func(g *gocui.Gui) error {
		v, err := g.View(m.name)
		if err != nil {
			log.Fatal(err)
		}
		v.Clear()
		for _, info := range sfs {
			fmt.Fprintf(v, "File: %20.20s \t| Chunks: %5d \t| ToDownload: %5d\n", Truncate(info.name, 20), info.chunkCount, info.toDownloadCount)
		}
		return nil
	})
}
