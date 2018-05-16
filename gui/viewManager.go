package ui

import (
	"github.com/jroimartin/gocui"
)

type ViewManager struct {
	name           string
	title          string
	editable       bool
	wrap           bool
	autoscroll     bool
	x0, y0, x1, y1 int
}

// implement gocui manager interface
func (m ViewManager) Layout(g *gocui.Gui) error {
	// initialize gui, once
	v, err := g.SetView(m.name, m.x0, m.y0, m.x1, m.y1)
	if err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		// configure view
		v.Title = m.title
		v.Editable = m.editable
		v.Wrap = m.wrap
		v.Autoscroll = m.autoscroll
	}
	return nil
}

func (m ViewManager) Name() string {
	return m.name
}
