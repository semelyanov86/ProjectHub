#!/usr/bin/env python3
"""Reads and clicks the dbusmenu of a StatusNotifierItem, like a tray host does.

Usage: tray_client.py <bus-name> labels
       tray_client.py <bus-name> click <label>
"""
import sys

from gi.repository import Gio, GLib

bus = Gio.bus_get_sync(Gio.BusType.SESSION, None)
name = sys.argv[1]


def layout():
    reply = bus.call_sync(name, "/StatusNotifierMenu", "com.canonical.dbusmenu", "GetLayout",
                          GLib.Variant("(iias)", (0, -1, [])), None, Gio.DBusCallFlags.NONE, 5000, None)
    _revision, root = reply.unpack()
    entries = []

    def walk(node):
        item_id, props, children = node
        if "label" in props:
            entries.append((item_id, props["label"], props.get("enabled", True)))
        for child in children:
            walk(child)

    walk(root)
    return entries


if sys.argv[2] == "labels":
    for item_id, label, enabled in layout():
        print(f"{item_id}\t{'enabled' if enabled else 'disabled'}\t{label}")
elif sys.argv[2] == "click":
    target = sys.argv[3]
    matches = [item_id for item_id, label, _ in layout() if label == target]
    if not matches:
        sys.exit(f"no menu item labelled {target!r}")
    bus.call_sync(name, "/StatusNotifierMenu", "com.canonical.dbusmenu", "Event",
                  GLib.Variant("(isvu)", (matches[0], "clicked", GLib.Variant("i", 0), 0)),
                  None, Gio.DBusCallFlags.NONE, 5000, None)
    print(f"clicked {target}")
