package tray

import (
	"context"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	watcherName      = "org.kde.StatusNotifierWatcher"
	watcherPath      = dbus.ObjectPath("/StatusNotifierWatcher")
	hostCheckTimeout = 500 * time.Millisecond
)

// HostAvailable reports whether the session has a StatusNotifierItem host
// that can display ProjectHub's tray icon. Stock GNOME has none; Ubuntu's
// default session provides one through the Ubuntu AppIndicators extension,
// KDE Plasma and most other desktops provide one natively. Any D-Bus failure
// is treated as "no tray" so callers fall back to behaviour that never leaves
// the application running invisibly.
func HostAvailable(ctx context.Context) bool {
	conn, err := dbus.SessionBus()
	if err != nil {
		return false
	}
	return hostAvailable(ctx, conn)
}

func hostAvailable(ctx context.Context, conn *dbus.Conn) bool {
	ctx, cancel := context.WithTimeout(ctx, hostCheckTimeout)
	defer cancel()

	var owned bool
	err := conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.NameHasOwner", 0, watcherName).Store(&owned)
	if err != nil || !owned {
		return false
	}
	var registered dbus.Variant
	err = conn.Object(watcherName, watcherPath).
		CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, watcherName, "IsStatusNotifierHostRegistered").
		Store(&registered)
	if err != nil {
		return false
	}
	value, ok := registered.Value().(bool)
	return ok && value
}
