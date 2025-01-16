package schema

import (
	"log/slog"
	"strings"

	"github.com/fsnotify/fsnotify"

	"seer/interfaces"
)

func WatchNewABIs(dir string, parser interfaces.ABIParser) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error("error creating abi directory watcher", "error", err)
		return
	}
	defer func(watcher *fsnotify.Watcher) {
		err := watcher.Close()
		if err != nil {
			slog.Error("Error closing watcher", "error", err)
			return
		}
	}(watcher)

	err = watcher.Add(dir)
	if err != nil {
		slog.Error("error closing abi directory watcher", "error", err)
		return
	}
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				slog.Error("exiting abi directory watcher", "error", err)
				return
			}
			if event.Op.Has(fsnotify.Create|fsnotify.Write) && strings.HasSuffix(event.Name, ".abi") {
				slog.Info("New abi detected", "name", event.Name)
				err = parser.Parse(event.Name)
				if err != nil {
					slog.Error("Error parsing new abi", "file name", event.Name, "error", err)
					continue
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				slog.Error("unknown watcher error")
				return
			}
			slog.Error("Watcher error", "error", err)
		}
	}
}
