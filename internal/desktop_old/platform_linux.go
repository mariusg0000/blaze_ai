// platform_linux.go — Linux tray, hotkey, and native window integration.
// Hooks the embedded webview window into GTK so the desktop transport can hide
// to tray, restore from tray or hotkey, and persist native window geometry.
// Layer: transport platform integration. Dependencies: gtk3, systray, hotkey, webview.
//go:build linux

package desktop

/*
#cgo pkg-config: gtk+-3.0 gdk-pixbuf-2.0
#include <stdint.h>
#include <gtk/gtk.h>
#include <gio/gio.h>
#include <gdk-pixbuf/gdk-pixbuf.h>

extern gboolean desktopDeleteEvent(GtkWidget* widget, GdkEvent* event, gpointer data);
extern gboolean desktopConfigureEvent(GtkWidget* widget, GdkEvent* event, gpointer data);

static void desktopInstallSignals(GtkWindow* window, uintptr_t data) {
	g_signal_connect(G_OBJECT(window), "delete-event", G_CALLBACK(desktopDeleteEvent), (gpointer)data);
	g_signal_connect(G_OBJECT(window), "configure-event", G_CALLBACK(desktopConfigureEvent), (gpointer)data);
}

static void desktopShowWindow(GtkWindow* window) {
	gtk_widget_show_all(GTK_WIDGET(window));
	gtk_window_present(window);
}

static void desktopHideWindow(GtkWindow* window) {
	gtk_widget_hide(GTK_WIDGET(window));
}

static char* desktopPickDirectory(const char* title, const char* defaultPath, int* outMode) {
	GtkWidget* dialog = gtk_file_chooser_dialog_new(
		title, NULL, GTK_FILE_CHOOSER_ACTION_SELECT_FOLDER,
		"New Session", 1,
		"Last Session", 2,
		"Cancel", GTK_RESPONSE_CANCEL,
		NULL
	);
	if (defaultPath != NULL && g_file_test(defaultPath, G_FILE_TEST_IS_DIR)) {
		gtk_file_chooser_set_filename(GTK_FILE_CHOOSER(dialog), defaultPath);
	}
	gchar* result = NULL;
	gint response = gtk_dialog_run(GTK_DIALOG(dialog));
	if (outMode != NULL) *outMode = response;
	if (response == 1 || response == 2) {
		result = gtk_file_chooser_get_filename(GTK_FILE_CHOOSER(dialog));
	}
	gtk_widget_destroy(dialog);
	return result;
}

static gboolean desktopConfirmDialog(const char* title, const char* message) {
	GtkWidget* dialog = gtk_message_dialog_new(
		NULL, GTK_DIALOG_MODAL,
		GTK_MESSAGE_QUESTION, GTK_BUTTONS_OK_CANCEL,
		"%s", message
	);
	gtk_window_set_title(GTK_WINDOW(dialog), title);
	gint result = gtk_dialog_run(GTK_DIALOG(dialog));
	gtk_widget_destroy(dialog);
	return result == GTK_RESPONSE_OK;
}

static gboolean desktopWindowVisible(GtkWindow* window) {
	return gtk_widget_get_visible(GTK_WIDGET(window));
}

static void desktopMoveAndResizeWindow(GtkWindow* window, int x, int y, int width, int height) {
	gtk_window_move(window, x, y);
	gtk_window_resize(window, width, height);
}

static void desktopGetWindowGeometry(GtkWindow* window, int* x, int* y, int* width, int* height) {
	gtk_window_get_position(window, x, y);
	gtk_window_get_size(window, width, height);
}

static gboolean desktopSetWindowIcon(GtkWindow* window, const guint8* data, gsize len, char** error_message) {
	GInputStream* stream = g_memory_input_stream_new_from_data(data, len, NULL);
	GError* err = NULL;
	GdkPixbuf* pixbuf = gdk_pixbuf_new_from_stream(stream, NULL, &err);
	g_object_unref(stream);
	if (pixbuf == NULL) {
		if (err != NULL && error_message != NULL) {
			*error_message = g_strdup(err->message);
			g_error_free(err);
		}
		return FALSE;
	}
	gtk_window_set_icon(window, pixbuf);
	g_object_unref(pixbuf);
	return TRUE;
}
*/
import "C"

import (
	"fmt"
	"runtime/cgo"
	"sync"
	"unsafe"

	"github.com/getlantern/systray"
	webview "github.com/webview/webview_go"
	hotkey "golang.design/x/hotkey"

	"blazeai/internal/platform"
)

type desktopPlatform interface {
	Shutdown()
}

type linuxDesktopPlatform struct {
	ui         *desktopUI
	view       webview.WebView
	window     *linuxWindowController
	hotkey     *hotkey.Hotkey
	hotkeySpec HotkeySpec
	quitOnce   sync.Once
	stopOnce   sync.Once
}

type linuxWindowController struct {
	view        webview.WebView
	window      *C.GtkWindow
	handle      cgo.Handle
	onDelete    func(WindowBounds)
	onConfigure func(WindowBounds)
}

func startDesktopPlatform(view webview.WebView, ui *desktopUI, cfg *Config, osType platform.OS) (desktopPlatform, error) {
	if osType != platform.Linux {
		return nil, fmt.Errorf("desktop tray, hotkey, and window persistence are supported only on Linux right now")
	}
	platformUI := &linuxDesktopPlatform{ui: ui, view: view}
	iconBytes, err := desktopIconPNG()
	if err != nil {
		return nil, err
	}
	controller, err := newLinuxWindowController(view, iconBytes, platformUI.onDeleteRequested, platformUI.onConfigureChanged)
	if err != nil {
		return nil, err
	}
	platformUI.window = controller
	if bounds, ok := ui.state.WindowBoundsValue(); ok {
		if err := controller.ApplyBounds(bounds); err != nil {
			return nil, err
		}
	}
	systray.Register(platformUI.onTrayReady, nil)
	spec, err := ParseHotkeyConfig(cfg.ToggleHotkey)
	if err != nil {
		return nil, err
	}
	if cfg.ToggleHotkey.Enabled {
		registeredHotkey := hotkey.New(spec.Modifiers, spec.Key)
		if err := registeredHotkey.Register(); err != nil {
			return nil, fmt.Errorf("cannot register desktop toggle hotkey %s: %w", spec.Display, err)
		}
		platformUI.hotkey = registeredHotkey
		platformUI.hotkeySpec = spec
		go platformUI.runHotkeyLoop()
	}
	return platformUI, nil
}

func (p *linuxDesktopPlatform) Shutdown() {
	p.stopOnce.Do(func() {
		if p.hotkey != nil {
			_ = p.hotkey.Unregister()
		}
		if p.window != nil {
			p.window.Close()
		}
		_ = p.ui.flushState()
		systray.Quit()
	})
}

// pickDirectoryNative opens a GTK directory chooser with three buttons:
// New Session (mode=1), Last Session (mode=2), Cancel (mode=0).
// Must be called from within view.Dispatch or another GTK main thread context.
func pickDirectoryNative(title, defaultPath string) (path string, mode int, err error) {
	cTitle := C.CString(title)
	cDefault := C.CString(defaultPath)
	defer C.free(unsafe.Pointer(cTitle))
	defer C.free(unsafe.Pointer(cDefault))
	var outMode C.int
	cPath := C.desktopPickDirectory(cTitle, cDefault, &outMode)
	mode = int(outMode)
	if cPath == nil {
		return "", mode, nil
	}
	defer C.g_free(C.gpointer(cPath))
	return C.GoString(cPath), mode, nil
}

// confirmDialog shows a GTK OK/Cancel question dialog.
// Returns true if the user clicked OK.
func confirmDialog(title, message string) (bool, error) {
	cTitle := C.CString(title)
	cMessage := C.CString(message)
	defer C.free(unsafe.Pointer(cTitle))
	defer C.free(unsafe.Pointer(cMessage))
	result := C.desktopConfirmDialog(cTitle, cMessage)
	return result == C.TRUE, nil
}

func (p *linuxDesktopPlatform) onTrayReady() {
	iconBytes, err := desktopIconPNG()
	if err != nil {
		p.ui.AppendSystem("Error: " + err.Error())
		return
	}
	systray.SetIcon(iconBytes)
	systray.SetTitle("BlazeAI")
	showItem := systray.AddMenuItem("Show BlazeAI", "Show the desktop window")
	hideItem := systray.AddMenuItem("Hide BlazeAI", "Hide the desktop window to tray")
	systray.AddSeparator()
	quitItem := systray.AddMenuItem("Quit BlazeAI", "Quit the desktop app")
	go func() {
		for {
			select {
			case <-showItem.ClickedCh:
				if err := p.showWindow(); err != nil {
					p.ui.AppendSystem("Error: " + err.Error())
				}
			case <-hideItem.ClickedCh:
				if err := p.hideWindow(); err != nil {
					p.ui.AppendSystem("Error: " + err.Error())
				}
			case <-quitItem.ClickedCh:
				p.quitOnce.Do(func() { p.ui.requestQuit("Desktop app is shutting down.") })
				return
			case <-p.ui.quitCh:
				return
			}
		}
	}()
}

func (p *linuxDesktopPlatform) runHotkeyLoop() {
	for {
		select {
		case <-p.hotkey.Keydown():
			if err := p.toggleWindow(); err != nil {
				p.ui.AppendSystem("Error: " + err.Error())
			}
		case <-p.ui.quitCh:
			return
		}
	}
}

func (p *linuxDesktopPlatform) onDeleteRequested(bounds WindowBounds) {
	p.ui.rememberWindowBounds(bounds)
	_ = p.ui.flushState()
}

func (p *linuxDesktopPlatform) onConfigureChanged(bounds WindowBounds) {
	p.ui.rememberWindowBounds(bounds)
}

func (p *linuxDesktopPlatform) toggleWindow() error {
	visible, err := p.window.Visible()
	if err != nil {
		return err
	}
	if visible {
		return p.hideWindow()
	}
	return p.showWindow()
}

func (p *linuxDesktopPlatform) showWindow() error {
	return p.window.Show()
}

func (p *linuxDesktopPlatform) hideWindow() error {
	bounds, err := p.window.Bounds()
	if err != nil {
		return err
	}
	p.ui.rememberWindowBounds(bounds)
	if err := p.ui.flushState(); err != nil {
		return err
	}
	return p.window.Hide()
}

func newLinuxWindowController(view webview.WebView, iconBytes []byte, onDelete func(WindowBounds), onConfigure func(WindowBounds)) (*linuxWindowController, error) {
	rawWindow := view.Window()
	if rawWindow == nil {
		return nil, fmt.Errorf("cannot resolve native GTK window for desktop transport")
	}
	controller := &linuxWindowController{
		view:        view,
		window:      (*C.GtkWindow)(rawWindow),
		onDelete:    onDelete,
		onConfigure: onConfigure,
	}
	controller.handle = cgo.NewHandle(controller)
	C.desktopInstallSignals(controller.window, C.uintptr_t(controller.handle))
	if err := controller.SetIcon(iconBytes); err != nil {
		controller.handle.Delete()
		return nil, err
	}
	return controller, nil
}

func (c *linuxWindowController) Close() {
	if c.handle != 0 {
		c.handle.Delete()
		c.handle = 0
	}
}

func (c *linuxWindowController) SetIcon(iconBytes []byte) error {
	var cMessage *C.char
	ok := C.desktopSetWindowIcon(c.window, (*C.guint8)(unsafe.Pointer(&iconBytes[0])), C.gsize(len(iconBytes)), &cMessage)
	if ok != C.FALSE {
		return nil
	}
	if cMessage != nil {
		defer C.g_free(C.gpointer(cMessage))
		return fmt.Errorf("cannot set desktop window icon: %s", C.GoString(cMessage))
	}
	return fmt.Errorf("cannot set desktop window icon")
}

func (c *linuxWindowController) ApplyBounds(bounds WindowBounds) error {
	C.desktopMoveAndResizeWindow(c.window, C.int(bounds.X), C.int(bounds.Y), C.int(bounds.Width), C.int(bounds.Height))
	return nil
}

func (c *linuxWindowController) Show() error {
	return c.dispatchSync(func() error {
		C.desktopShowWindow(c.window)
		return nil
	})
}

func (c *linuxWindowController) Hide() error {
	return c.dispatchSync(func() error {
		C.desktopHideWindow(c.window)
		return nil
	})
}

func (c *linuxWindowController) Visible() (bool, error) {
	result := make(chan bool, 1)
	errCh := make(chan error, 1)
	c.view.Dispatch(func() {
		result <- C.desktopWindowVisible(c.window) != C.FALSE
		errCh <- nil
	})
	if err := <-errCh; err != nil {
		return false, err
	}
	return <-result, nil
}

func (c *linuxWindowController) Bounds() (WindowBounds, error) {
	result := make(chan WindowBounds, 1)
	errCh := make(chan error, 1)
	c.view.Dispatch(func() {
		result <- c.boundsDirect()
		errCh <- nil
	})
	if err := <-errCh; err != nil {
		return WindowBounds{}, err
	}
	return <-result, nil
}

func (c *linuxWindowController) boundsDirect() WindowBounds {
	var x, y, width, height C.int
	C.desktopGetWindowGeometry(c.window, &x, &y, &width, &height)
	return WindowBounds{X: int(x), Y: int(y), Width: int(width), Height: int(height)}
}

func (c *linuxWindowController) dispatchSync(fn func() error) error {
	errCh := make(chan error, 1)
	c.view.Dispatch(func() {
		errCh <- fn()
	})
	return <-errCh
}

//export desktopDeleteEvent
func desktopDeleteEvent(widget *C.GtkWidget, event *C.GdkEvent, data C.gpointer) C.gboolean {
	handle := cgo.Handle(uintptr(data))
	controller := handle.Value().(*linuxWindowController)
	bounds := controller.boundsDirect()
	if controller.onDelete != nil {
		controller.onDelete(bounds)
	}
	C.desktopHideWindow(controller.window)
	return C.TRUE
}

//export desktopConfigureEvent
func desktopConfigureEvent(widget *C.GtkWidget, event *C.GdkEvent, data C.gpointer) C.gboolean {
	handle := cgo.Handle(uintptr(data))
	controller := handle.Value().(*linuxWindowController)
	if controller.onConfigure != nil {
		controller.onConfigure(controller.boundsDirect())
	}
	return C.FALSE
}
