// SPDX-License-Identifier: MIT

package sources

import (
	"strings"
	"testing"
)

func TestArduinoAVRCore(t *testing.T) {
	// what arduino-cli installs for arduino:avr@1.8.6
	items, err := ParseArduino([]string{
		"arduino/hardware/avr/1.8.6",
		"arduino/tools/avr-gcc/7.3.0-atmel3.6.1-arduino7",
		"arduino/tools/avrdude/6.3.0-arduino17",
		"arduino/tools/arduinoOTA/1.3.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := Arduino(items)
	if err != nil {
		t.Fatal(err)
	}
	got := FormatArduino(entries)
	for _, want := range []string{
		"avr 1.8.6 (LGPL-2.1)  https://github.com/arduino/ArduinoCore-avr/tree/1.8.6\n",
		"avr-gcc 7.3.0-atmel3.6.1-arduino7 (GPL-3.0)  https://github.com/arduino/toolchain-avr/tree/7.3.0-atmel3.6.1-arduino7\n",
		"    GCC and binutils GPL-3.0; avr-libc",
		"avrdude 6.3.0-arduino17 (GPL-2.0)  https://github.com/avrdudes/avrdude/tree/v6.3 (Arduino's build: https://github.com/arduino/avrdude-build-script)\n",
		"arduinoOTA 1.3.0 (GPL-3.0)  https://github.com/arduino/arduinoOTA/tree/1.3.0\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	if l := strings.Join(Licenses(entries), " "); l != "GPL-2.0 GPL-3.0 LGPL-2.1" {
		t.Errorf("licences %q", l)
	}
}

func TestArduinoBuiltinTools(t *testing.T) {
	items, err := ParseArduino([]string{
		"builtin/tools/ctags/5.8-arduino11",
		"builtin/tools/serial-discovery/1.4.1",
		"builtin/tools/mdns-discovery/1.0.9",
		"builtin/tools/serial-monitor/0.14.1",
		"builtin/tools/dfu-discovery/0.1.2",
	})
	if err != nil {
		t.Fatal(err)
	}
	items = append(items, Item{Vendor: "arduino", Kind: "cli", Name: "arduino-cli", Version: "1.6.0"})
	entries, err := Arduino(items)
	if err != nil {
		t.Fatal(err)
	}
	got := FormatArduino(entries)
	for _, want := range []string{
		"arduino-cli 1.6.0 (GPL-3.0)  https://github.com/arduino/arduino-cli/tree/v1.6.0\n",
		"ctags 5.8-arduino11 (GPL-2.0)  https://github.com/arduino/ctags/tree/5.8-arduino11\n",
		"serial-discovery 1.4.1 (GPL-3.0)  https://github.com/arduino/serial-discovery/tree/v1.4.1\n",
		"dfu-discovery 0.1.2 (GPL-3.0)  https://github.com/arduino/dfu-discovery/tree/v0.1.2\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	for _, l := range Licenses(entries) {
		if CommonLicense[l] == "" {
			t.Errorf("%s has no common-licenses text", l)
		}
	}
}

func TestArduinoUnknownToolFails(t *testing.T) {
	items, _ := ParseArduino([]string{"arduino/tools/bossac/1.9.1"})
	if _, err := Arduino(items); err == nil || !strings.Contains(err.Error(), "bossac") {
		t.Fatalf("want an error naming bossac, got %v", err)
	}
}

func TestParseArduinoRejectsOtherPaths(t *testing.T) {
	for _, bad := range []string{"arduino/tools/avr-gcc", "arduino/staging/x/1", "a/tools/b/c/d"} {
		if _, err := ParseArduino([]string{bad}); err == nil {
			t.Errorf("%s: want an error", bad)
		}
	}
}

func TestAvrdude7(t *testing.T) {
	if got := avrdude("7.1-arduino.1"); got != "https://github.com/avrdudes/avrdude/tree/v7.1 (Arduino's build: https://github.com/arduino/avrdude-packing)" {
		t.Error(got)
	}
}

func TestDebLine(t *testing.T) {
	if got := DebLine("libusb-1.0-0:amd64", "2:1.0.27-1", "libusb-1.0", "2:1.0.27-1"); got != "libusb-1.0-0:amd64 2:1.0.27-1  https://launchpad.net/ubuntu/+source/libusb-1.0/2:1.0.27-1" {
		t.Error(got)
	}
	if got := DebLine("libmpc3", "1.3.1-1", "", ""); got != "libmpc3 1.3.1-1  https://launchpad.net/ubuntu/+source/libmpc3/1.3.1-1" {
		t.Error(got)
	}
}

func TestParseDpkgSearch(t *testing.T) {
	pkg, err := ParseDpkgSearch("libusb-1.0-0:amd64: /usr/lib/x86_64-linux-gnu/libusb-1.0.so.0\n")
	if err != nil || pkg != "libusb-1.0-0:amd64" {
		t.Fatalf("%q %v", pkg, err)
	}
	// libreadline on 24.04: the /lib path has only diversions, the /usr/lib path the owner too
	pkg, err = ParseDpkgSearch("diversion by libreadline8t64 from: /usr/lib/x/libreadline.so.8\nlibreadline8t64:amd64: /usr/lib/x/libreadline.so.8\n")
	if err != nil || pkg != "libreadline8t64:amd64" {
		t.Fatalf("%q %v", pkg, err)
	}
	for _, bad := range []string{"", "a:amd64, b:amd64: /usr/lib/x", "diversion by foo from: /x\ndiversion by foo to: /x.usr-is-merged\n", "x: /a\ny: /b"} {
		if _, err := ParseDpkgSearch(bad); err == nil {
			t.Errorf("%q: want an error", bad)
		}
	}
}
