package ui

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/jroimartin/gocui"
	"github.com/monz/fastSharerGo/net/data"
	"log"
	"sync"
)

type NodeViewMgr struct {
	ViewManager // inherit ViewManager
	nodes       map[uuid.UUID]*data.Node
	mu          sync.Mutex
}

func NewNodeViewMgr(name, title string, editable, wrap, autoscroll bool, x0, y0, x1, y1 int) *NodeViewMgr {
	m := new(NodeViewMgr)
	m.name = name
	m.title = title
	m.editable = editable
	m.wrap = wrap
	m.autoscroll = autoscroll
	m.x0 = x0
	m.y0 = y0
	m.x1 = x1
	m.y1 = y1
	m.nodes = make(map[uuid.UUID]*data.Node)

	return m
}

func (m *NodeViewMgr) Update(g *gocui.Gui, node *data.Node) {
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
		for _, node := range m.nodes {
			for _, ip := range node.Ips() {
				fmt.Fprintf(v, "Node: %36.36s \t| Ip: %15.15s\n", Truncate(node.Id().String(), 36), Truncate(ip, 15))
			}
		}
		return nil
	})
}
