package local

import (
	"github.com/fsnotify/fsnotify"
	"github.com/monz/fastSharerGo/common/data"
	"log"
	"os"
	"path/filepath"
	"sync"
)

type FileDiscoveryService struct {
	watchedDirectories map[string]bool
	watcher            []*fsnotify.Watcher
	subscriber         []data.DirectoryChangeSubscriber
	mu                 sync.Mutex
}

func NewFileDiscoveryService(dir string, recursive bool) *FileDiscoveryService {
	f := new(FileDiscoveryService)
	f.watchedDirectories = make(map[string]bool)
	// add valid path
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		log.Println("Directory does not exist")
	} else if err == nil {
		f.watchedDirectories[dir] = recursive
	}

	return f
}

// implement service interface
func (f *FileDiscoveryService) Start() {
	f.watchDirectories()
}

// implement service interface
func (f *FileDiscoveryService) Stop() {
	// close all watcher
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, w := range f.watcher {
		w.Close()
	}
}

func (f *FileDiscoveryService) Register(subscriber data.DirectoryChangeSubscriber) {
	f.subscriber = append(f.subscriber, subscriber)
}

func (f *FileDiscoveryService) AddDirectory(dir string, recursive bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// only add valid paths
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		log.Println("Directory does not exist")
		return
	}
	_, ok := f.watchedDirectories[dir]
	if ok {
		// already watching given directory, nothing to do
		return
	}
	// add new directory
	f.watchedDirectories[dir] = recursive

	f.watchDirectory(dir, recursive)
}

func (f *FileDiscoveryService) RemoveDirectory(dir string) {
	// todo: implement
}

func (f *FileDiscoveryService) watchDirectory(dir string, recursive bool) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	f.watcher = append(f.watcher, watcher)

	if recursive {
		// watch directories recursively
		err := filepath.Walk(dir, func(path string, fi os.FileInfo, _ error) error {
			if fi.Mode().IsDir() {
				return watcher.Add(path)
			}
			return nil
		})
		if err != nil {
			return err
		}
	} else {
		// watch just given directory
		err := watcher.Add(dir)
		if err != nil {
			return err
		}
	}
	// start watching
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				// do something with event
				log.Printf("EVENT! %#v\n", event)
			case err := <-watcher.Errors:
				// handle errors
				log.Println("ERROR", err)
			}
		}
	}()
	return nil
}

func (f *FileDiscoveryService) WatchedDirectories() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var watched []string
	for dir, _ := range f.watchedDirectories {
		watched = append(watched, dir)
	}
	return watched
}

func (f FileDiscoveryService) UpdateDirectoryChange(sf data.SharedFile) {
	for _, s := range f.subscriber {
		s.AddLocalSharedFile(sf)
	}
}

func (f *FileDiscoveryService) watchDirectories() {
	// close all existing watchers
	f.Stop()
	// cannot lock befoer, because f.Stop() will also lock 'mu'
	f.mu.Lock()
	defer f.mu.Unlock()
	// start watching directories
	for dir, recursive := range f.watchedDirectories {
		f.watchDirectory(dir, recursive)
	}
}
