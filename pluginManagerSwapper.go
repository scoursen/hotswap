package hotswap

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/scoursen/hotswap/internal/hutils"
	"github.com/edwingeng/slog"
)

var (
	minFreeDelay = time.Second * 15
)

type ReloadCallback func(newManager, oldManager *PluginManager) error

type pluginWhitelist []string

func (pw pluginWhitelist) Contains(name string) bool {
	for _, v := range pw {
		if v == name {
			return true
		}
	}
	return false
}

type PluginManagerSwapper struct {
	slog.Logger
	current atomic.Value

	opts struct {
		pluginDir      string
		newExt         func() interface{}
		reloadCallback ReloadCallback
		freeDelay      time.Duration
		whitelist      pluginWhitelist
	}

	staticPlugins map[string]*StaticPlugin
	reloadCounter int64

	mu sync.Mutex
}

func NewPluginManagerSwapper(pluginDir string, opts ...Option) *PluginManagerSwapper {
	swapper := &PluginManagerSwapper{Logger: slog.NewDevelopmentConfig().MustBuild()}
	swapper.opts.pluginDir = pluginDir
	swapper.opts.freeDelay = time.Minute * 5
	for _, opt := range opts {
		opt(swapper)
	}
	return swapper
}

func (sw *PluginManagerSwapper) ResetPluginDir(pluginDir string) {
	sw.opts.pluginDir = pluginDir
}

func (sw *PluginManagerSwapper) Current() *PluginManager {
	v := sw.current.Load()
	pluginManager, _ := v.(*PluginManager)
	return pluginManager
}

func (sw *PluginManagerSwapper) LoadPlugins(data interface{}) (Details, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	cbs := []ReloadCallback{sw.opts.reloadCallback}
	if sw.staticPlugins != nil {
		return sw.loadStaticPlugins(data, cbs)
	}

	return sw.loadPluginsImpl(data, cbs)
}

func (sw *PluginManagerSwapper) loadPluginsImpl(data interface{}, cbs []ReloadCallback) (Details, error) {
	var absDir string
	if err := hutils.FindDirectory(sw.opts.pluginDir, "pluginDir"); err != nil {
		return nil, err
	} else if absDir, err = filepath.Abs(sw.opts.pluginDir); err != nil {
		return nil, err
	}

	a, err := ioutil.ReadDir(absDir)
	if err != nil {
		return nil, err
	}
	var files []string
	var found = make(map[string]struct{})
	for _, fi := range a {
		if fi.IsDir() {
			continue
		}
		if strings.HasSuffix(fi.Name(), hutils.FileNameExt) {
			if len(sw.opts.whitelist) > 0 {
				if name := pluginName(fi.Name()); sw.opts.whitelist.Contains(name) {
					found[name] = struct{}{}
				} else {
					continue
				}
			}
			files = append(files, filepath.Join(absDir, fi.Name()))
		}
	}
	if len(sw.opts.whitelist) > 0 {
		if len(found) != len(sw.opts.whitelist) {
			var missing []string
			for _, v := range sw.opts.whitelist {
				if _, ok := found[v]; !ok {
					missing = append(missing, v)
				}
			}
			return nil, errors.New("cannot find the following plugin(s): " + hutils.Join(missing...))
		}
	}

	return sw.loadPluginFiles(files, data, cbs)
}

func (sw *PluginManagerSwapper) loadPluginFiles(files []string, data interface{}, cbs []ReloadCallback) (Details, error) {
	if len(files) == 0 {
		return nil, nil
	}

	oldManager := sw.Current()
	newManager := newPluginManager(sw.Logger, sw.opts.newExt)
	if err := newManager.loadPlugins(files, oldManager, data); err != nil {
		return nil, err
	}
	if err := invokeReloadCallbacks(cbs, newManager, oldManager); err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, f := range files {
		p := newManager.FindPlugin(pluginName(f))
		if p.Note != "" {
			result[p.File] = p.Note
		} else {
			result[p.File] = "ok"
		}
	}
	if oldManager != nil {
		go func() {
			delay := minFreeDelay
			if minFreeDelay < sw.opts.freeDelay {
				delay = sw.opts.freeDelay
			}
			time.Sleep(delay)
			oldManager.invokeEveryOnFree()
		}()
	}

	sw.current.Store(newManager)
	return result, nil
}

func invokeReloadCallbacks(cbs []ReloadCallback, newManager, oldManager *PluginManager) error {
	for _, cb := range cbs {
		if cb == nil {
			continue
		}
		err := func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("<hotswap> panic: %+v\n%s", r, debug.Stack())
				}
			}()
			return cb(newManager, oldManager)
		}()
		if err != nil {
			newManager.invokeEveryOnFree()
			return err
		}
	}
	return nil
}

func (sw *PluginManagerSwapper) Reload(data interface{}) (Details, error) {
	return sw.ReloadWithCallback(data, nil)
}

func (sw *PluginManagerSwapper) ReloadWithCallback(data interface{}, extra ReloadCallback) (Details, error) {
	if sw.staticPlugins != nil {
		return nil, errors.New("running under static linking mode")
	}

	sw.mu.Lock()
	defer sw.mu.Unlock()
	cbs := []ReloadCallback{sw.opts.reloadCallback}
	if extra != nil {
		cbs = append(cbs, extra)
	}
	details, err := sw.loadPluginsImpl(data, cbs)
	if err == nil {
		atomic.AddInt64(&sw.reloadCounter, 1)
	}
	return details, err
}

func (sw *PluginManagerSwapper) ReloadCounter() int64 {
	return atomic.LoadInt64(&sw.reloadCounter)
}

func (sw *PluginManagerSwapper) StaticLinkingMode() bool {
	return sw.staticPlugins != nil
}

type Details map[string]string

func (d Details) String() string {
	var a []string
	for k := range d {
		a = append(a, k)
	}
	sort.Strings(a)

	var buf bytes.Buffer
	for i, k := range a {
		if i > 0 {
			_, _ = buf.WriteString(", ")
		}
		x := strings.TrimSuffix(filepath.Base(k), hutils.FileNameExt)
		_, _ = fmt.Fprintf(&buf, "%s: %s", x, d[k])
	}
	return buf.String()
}

type Option func(mgr *PluginManagerSwapper)

// WithLogger replaces the default logger with your own.
func WithLogger(log slog.Logger) Option {
	return func(mgr *PluginManagerSwapper) {
		mgr.Logger = log
	}
}

// WithFreeDelay sets the delay time of calling OnFree. The default value is 5 minutes.
func WithFreeDelay(d time.Duration) Option {
	return func(mgr *PluginManagerSwapper) {
		mgr.opts.freeDelay = d
	}
}

// WithReloadCallback sets the callback function of reloading.
func WithReloadCallback(cb ReloadCallback) Option {
	return func(mgr *PluginManagerSwapper) {
		mgr.opts.reloadCallback = cb
	}
}

// WithExtensionNewer sets the function used to create a new object for PluginManager.Vault.Extension.
func WithExtensionNewer(newExt func() interface{}) Option {
	return func(mgr *PluginManagerSwapper) {
		mgr.opts.newExt = newExt
	}
}

// WithStaticPlugins sets the static plugins for static linking.
func WithStaticPlugins(plugins map[string]*StaticPlugin) Option {
	return func(mgr *PluginManagerSwapper) {
		mgr.staticPlugins = plugins
	}
}

// WithWhitelist sets the plugins to load explicitly
func WithWhitelist(pluginNames ...string) Option {
	return func(mgr *PluginManagerSwapper) {
		mgr.opts.whitelist = pluginNames
	}
}
