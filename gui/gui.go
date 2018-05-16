package ui

import (
	"bytes"
	"fmt"
	"github.com/jroimartin/gocui"
	commonData "github.com/monz/fastSharerGo/common/data"
	"github.com/monz/fastSharerGo/net/data"
	"log"
	"time"
)

type Ui interface {
	Show(logBuffer *bytes.Buffer)
	Init() error
	// must implement interfaces
	data.ShareSubscriber
	data.NodeSubscriber
	commonData.DirectoryChangeSubscriber
}

type ShareUi struct {
	g                 *gocui.Gui
	sharedFileManager *SharedFileViewMgr
	nodeManager       *NodeViewMgr
	logManager        *LogViewMgr
}

func NewShareUi() *ShareUi {
	ui := new(ShareUi)
	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		log.Fatal("Could not create share ui")
	}
	ui.g = g
	return ui
}

func (ui *ShareUi) Show(logBuffer *bytes.Buffer) {
	// close ui when quitting
	defer ui.g.Close()

	// start main gui loop
	done := make(chan bool)
	go func() {
		if err := ui.g.MainLoop(); err != nil {
			if err != gocui.ErrQuit {
				log.Panicln(err)
			}
		}
		done <- true
	}()
	// only after main loop starts, views can be found
	// set global logger to log view
	time.Sleep(100 * time.Millisecond)
	v, err := ui.g.View(ui.logManager.Name())
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(v)
	// write saved log entries to log view
	ui.logManager.Update(ui.g, logBuffer)
	// wait until user quits gui
	<-done
}

func (ui *ShareUi) Init() error {
	maxX, maxY := ui.g.Size()
	ui.sharedFileManager = NewSharedFileViewMgr("sharedFiles", "Shared Files", false, true, true, 0, 0, maxX/2-1, maxY/2-1)
	ui.nodeManager = NewNodeViewMgr("nodes", "Nodes", false, true, true, maxX/2, 0, maxX-1, maxY/2-1)
	ui.logManager = NewLogViewMgr("log", "Log", false, true, true, 0, maxY/2, maxX-1, maxY-1)
	ui.g.SetManager(ui.sharedFileManager, ui.nodeManager, ui.logManager)

	return ui.setKeyBindings()
}

func (ui *ShareUi) setKeyBindings() error {
	if err := ui.g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		return err
	}
	return nil
}

func Truncate(s string, maxLength int) string {
	var sOut string
	if len(s) > maxLength {
		sOut = fmt.Sprintf("...%s", s[len(s)-(maxLength-3):])
	} else {
		sOut = s
	}
	return sOut
}

// implement share subscriber interface
func (ui *ShareUi) ReceivedDownloadRequest(r data.DownloadRequest) {
	// nothing to do
}

// implement share subscriber interface
func (ui *ShareUi) ReceivedDownloadRequestResult(rr data.DownloadRequestResult) {
	// nothing to do
}

// implement share subscriber interface
func (ui *ShareUi) ReceivedShareList(sf commonData.SharedFile) {
	// add missing files, only
	if _, ok := ui.sharedFileManager.sharedFiles[sf.FileId()]; !ok {
		ui.sharedFileManager.sharedFiles[sf.FileId()] = &sf
	}
	ui.sharedFileManager.Update(ui.g, &sf)
}

// implement directory change subscriber interface
func (ui *ShareUi) AddLocalSharedFile(sf commonData.SharedFile) {
	ui.sharedFileManager.Update(ui.g, &sf)
}

// implement node subscriber interface
func (ui *ShareUi) AddNode(n data.Node) {
	ui.nodeManager.mu.Lock()
	defer ui.nodeManager.mu.Unlock()

	// add missing nodes, only
	if _, ok := ui.nodeManager.nodes[n.Id()]; !ok {
		ui.nodeManager.nodes[n.Id()] = &n
	}
	ui.nodeManager.Update(ui.g, &n)
}

// implement node subscriber interface
func (ui *ShareUi) RemoveNode(n data.Node) {
	ui.nodeManager.mu.Lock()
	defer ui.nodeManager.mu.Unlock()

	// remove node if exists
	delete(ui.nodeManager.nodes, n.Id())
	ui.nodeManager.Update(ui.g, &n)
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}
