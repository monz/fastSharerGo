package ui

import (
	"bytes"
	"github.com/jroimartin/gocui"
	"io"
	"log"
	"sync"
)

type LogViewMgr struct {
	ViewManager // inherit ViewManager
	mu          sync.Mutex
}

func NewLogViewMgr(name, title string, editable, wrap, autoscroll bool, x0, y0, x1, y1 int) *LogViewMgr {
	m := new(LogViewMgr)
	m.name = name
	m.title = title
	m.editable = editable
	m.wrap = wrap
	m.autoscroll = autoscroll
	m.x0 = x0
	m.y0 = y0
	m.x1 = x1
	m.y1 = y1

	return m
}

func (m *LogViewMgr) Update(g *gocui.Gui, logBuffer *bytes.Buffer) {
	// create ui
	if err := m.Layout(g); err != nil {
		log.Fatal(err)
	}

	// update ui
	g.Update(func(g *gocui.Gui) error {
		v, err := g.View(m.name)
		if err != nil {
			log.Fatal(err)
		}
		v.Clear()
		io.Copy(v, logBuffer)
		//for _, node := range m.nodes {
		//	for _, ip := range node.Ips() {
		//		fmt.Fprintf(v, "Node: %36.36s \t| Ip: %15.15s\n", Truncate(node.Id().String(), 36), Truncate(ip, 15))
		//	}
		//}
		return nil
	})
}
