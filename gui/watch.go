package main

import (
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// watch calls changed (at most every 150 ms) when config.toml or the MangoHud config
// changes, e.g. through a keyboard shortcut running the CLI. It watches their folders, not
// the files: editors and the CLI's config save replace files, which ends a watch on the
// file itself. Folders that don't exist yet are tried again every few seconds. paths is
// asked again after each change, since config.toml can point to another MangoHud config.
func watch(paths func() []string, changed func(), stop <-chan struct{}) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	go func() {
		defer w.Close()
		watched := map[string]bool{}
		files := map[string]bool{}
		update := func() {
			files = map[string]bool{}
			for _, p := range paths() {
				files[filepath.Clean(p)] = true
				dir := filepath.Dir(p)
				if watched[dir] {
					continue
				}
				if info, err := os.Stat(dir); err == nil && info.IsDir() && w.Add(dir) == nil {
					watched[dir] = true
				}
			}
		}
		update()

		retry := time.NewTicker(3 * time.Second)
		defer retry.Stop()
		var debounce <-chan time.Time
		for {
			select {
			case <-stop:
				return
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				// Per-game configs (*.conf next to MangoHud.conf) matter too: the window
				// lists them
				if files[filepath.Clean(ev.Name)] || filepath.Ext(ev.Name) == ".conf" {
					debounce = time.After(150 * time.Millisecond)
				}
			case <-w.Errors:
			case <-retry.C:
				update()
			case <-debounce:
				debounce = nil
				update()
				changed()
			}
		}
	}()
	return nil
}
