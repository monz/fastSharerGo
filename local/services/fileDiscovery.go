package local

import (
	"errors"
	"github.com/fsnotify/fsnotify"
	"github.com/monz/fastSharerGo/common/data"
	"log"
	"os"
	"path/filepath"
	"sync"
)

type FileDiscoveryService struct {
	watchedDirectories map[string]bool
	watcher            *fsnotify.Watcher
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
	// start watching directory changes
	f.watchDirectories()
	// notify subscriber about already existing files/directories in shared directories
	for dir, recursive := range f.watchedDirectories {
		err := f.notifyExisting(dir, recursive)
		if err != nil {
			log.Println(err)
		}
	}
}

// implement service interface
func (f *FileDiscoveryService) Stop() {
	// close watcher
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.watcher != nil {
		f.watcher.Close()
		f.watcher = nil
	}
}

func (f *FileDiscoveryService) Register(subscriber data.DirectoryChangeSubscriber) {
	f.subscriber = append(f.subscriber, subscriber)
}

func (f *FileDiscoveryService) AddDirectory(dir string, recursive bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	// only add valid paths
	info, err := os.Stat(dir)
	if err != nil && os.IsNotExist(err) {
		return errors.New("Directory does not exist")
	}
	if !info.IsDir() {
		return errors.New("Given path is not a directory")
	}

	_, ok := f.watchedDirectories[dir]
	if ok {
		// already watching given directory, nothing to do
		return errors.New("Path gets already watched")
	}
	// add new directory
	f.watchedDirectories[dir] = recursive

	f.watchDirectory(dir, recursive)

	return nil
}

func (f *FileDiscoveryService) RemoveDirectory(dir string) {
	// todo: implement
}

func (f *FileDiscoveryService) watchDirectory(dir string, recursive bool) error {
	// create new watcher if none exist
	if f.watcher == nil {
		var err error
		f.watcher, err = fsnotify.NewWatcher()
		if err != nil {
			return err
		}
	}
	// add paths to watcher
	if recursive {
		// watch directories recursively
		err := filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if fi.Mode().IsDir() {
				return f.watcher.Add(path)
			}
			return nil
		})
		if err != nil {
			return err
		}
	} else {
		// watch just given directory
		err := f.watcher.Add(dir)
		if err != nil {
			return err
		}
	}
	// start watching
	go func() {
		for {
			select {
			case event := <-f.watcher.Events:
				f.handleWatcherEvent(event)
			case err := <-f.watcher.Errors:
				// todo: handle errors
				log.Println("ERROR", err)
			}
		}
	}()
	return nil
}

func (f *FileDiscoveryService) handleWatcherEvent(event fsnotify.Event) {
	switch event.Op {
	case fsnotify.Create:
		filePath := event.Name
		relativePath, err := filepath.Rel(filepath.Dir(filepath.Dir(filePath)), filePath)
		log.Println("Added file:", filePath, "relative path:", relativePath)
		if err != nil {
			log.Println(err)
		}
		err = f.handleFile(filePath, relativePath)
		if err != nil {
			log.Println(err)
		}
	case fsnotify.Write:
		// todo:
		log.Println("Write file:", event.Name)
	case fsnotify.Remove:
		// todo:
		log.Println("Remove file:", event.Name)
	case fsnotify.Rename:
		// todo:
		log.Println("Rename file:", event.Name)
	case fsnotify.Chmod:
		// todo:
		log.Println("Chmod file:", event.Name)
	default:
		log.Println("Unknown event, doesn't get handled!")
	}
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

func (f *FileDiscoveryService) updateDirectoryChange(sf data.SharedFile) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.subscriber {
		s.AddLocalSharedFile(sf)
	}
}

func (f *FileDiscoveryService) watchDirectories() {
	// close all existing watchers
	f.Stop()
	// cannot lock before, because f.Stop() will also lock 'mu'
	f.mu.Lock()
	defer f.mu.Unlock()
	// start watching directories
	for dir, recursive := range f.watchedDirectories {
		f.watchDirectory(dir, recursive)
	}
}

func (f *FileDiscoveryService) notifyExisting(dir string, recursive bool) error {
	base := filepath.Dir(dir)
	err := filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !fi.IsDir() {
			relativePath, err := filepath.Rel(base, path)
			if err != nil {
				return err
			}
			return f.handleFile(path, relativePath)
		} else if !recursive && fi.IsDir() && path != dir {
			// do not go down the directory hierarchy
			log.Println("Do not scan recursively, skipped dir:", path)
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return err
	}
	return err
}

func (f *FileDiscoveryService) handleFile(path string, relativePath string) error {
	// check if file exists
	info, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		return errors.New("File does not exist")
	}
	// do not handle directories
	if info.IsDir() {
		return errors.New("Is a directory!")
	}
	// create shared file object
	fileMetadata := data.NewFileMetadata(path, relativePath)
	sf := data.NewSharedFile(*fileMetadata)

	f.updateDirectoryChange(*sf)

	return nil
}
