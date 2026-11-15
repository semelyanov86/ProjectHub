#!/usr/bin/env python3
"""Minimal org.kde.StatusNotifierWatcher/host for headless tray tests.

It owns the watcher name on the session bus, reports that a host is
registered (as the GNOME AppIndicator extension or KDE Plasma would) and
prints every item that registers.
"""
from gi.repository import Gio, GLib

XML = """
<node>
  <interface name="org.kde.StatusNotifierWatcher">
    <method name="RegisterStatusNotifierItem"><arg type="s" direction="in"/></method>
    <method name="RegisterStatusNotifierHost"><arg type="s" direction="in"/></method>
    <property name="RegisteredStatusNotifierItems" type="as" access="read"/>
    <property name="IsStatusNotifierHostRegistered" type="b" access="read"/>
    <property name="ProtocolVersion" type="i" access="read"/>
  </interface>
</node>
"""
items = []


def method_call(_conn, sender, _path, _iface, method, params, invocation):
    if method == "RegisterStatusNotifierItem":
        service = params.unpack()[0]
        if service.startswith("/"):
            service = sender + service
        items.append(service)
        print(f"registered {service}", flush=True)
    invocation.return_value(None)


def get_property(_conn, _sender, _path, _iface, name):
    return {
        "RegisteredStatusNotifierItems": GLib.Variant("as", items),
        "IsStatusNotifierHostRegistered": GLib.Variant("b", True),
        "ProtocolVersion": GLib.Variant("i", 0),
    }[name]


def on_bus_acquired(conn, _name):
    node = Gio.DBusNodeInfo.new_for_xml(XML)
    conn.register_object("/StatusNotifierWatcher", node.interfaces[0], method_call, get_property, None)


def on_name_acquired(_conn, name):
    print(f"owned {name}", flush=True)


Gio.bus_own_name(Gio.BusType.SESSION, "org.kde.StatusNotifierWatcher", Gio.BusNameOwnerFlags.NONE,
                 on_bus_acquired, on_name_acquired, None)
GLib.MainLoop().run()
