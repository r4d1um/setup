# SPDX-License-Identifier: MIT
"""Stands in for an Arduino Uno's bootloader (optiboot, STK500 v1) on a pseudo-terminal, so the
tests run the real avrdude against it: it answers the commands avrdude's "arduino" programmer
sends and keeps what it is given in `flash` (byte address -> value).

Shared fake from r4d1um/setup (toolchain "fakes"). In Python: `from r4fakes.optiboot import FakeOptiboot`.
From any language: `fake-optiboot --port-file p --image-out img.bin` writes the pseudo-terminal's
path to p (temporary name, then renamed) and, when stopped with SIGTERM/SIGINT, the flash image."""
import os
import termios
import threading

OK, INSYNC, EOP = 0x10, 0x14, 0x20
SIGNATURE = bytes([0x1E, 0x95, 0x0F])  # ATmega328P


class FakeOptiboot:
    def __init__(self, signature: bytes = SIGNATURE, baud: int = 0):
        self.master, self.slave = os.openpty()
        self.port = os.ttyname(self.slave)
        self.signature = signature
        # baud: only answer when avrdude opened the port at this speed (old Nano bootloader: 57600)
        self.speed = {57600: termios.B57600, 115200: termios.B115200}.get(baud)
        self.flash: dict[int, int] = {}
        self.addr = 0
        self.stop = False
        self.thread = threading.Thread(target=self.run, daemon=True)
        self.thread.start()

    def close(self):
        self.stop = True
        for fd in (self.master, self.slave):
            try:
                os.close(fd)
            except OSError:
                pass

    def read(self, n: int) -> bytes:
        out = b""
        while len(out) < n:
            chunk = os.read(self.master, n - len(out))
            if not chunk:
                raise EOFError
            out += chunk
        return out

    def reply(self, data: bytes = b""):
        os.write(self.master, bytes([INSYNC]) + data + bytes([OK]))

    def image(self) -> bytes:
        """The flash from address 0 to the last byte written."""
        if not self.flash:
            return b""
        return bytes(self.flash.get(a, 0xFF) for a in range(max(self.flash) + 1))

    def run(self):
        try:
            while not self.stop:
                cmd = self.read(1)[0]
                if self.speed is not None and termios.tcgetattr(self.master)[4] != self.speed:
                    continue  # wrong speed: to the bootloader this is only noise
                if cmd == 0x41:                      # GET_PARAMETER
                    p = self.read(2)[0]
                    self.reply(bytes([{0x81: 8, 0x82: 3}.get(p, 3)]))
                elif cmd == 0x42:                    # SET_DEVICE
                    self.read(21)
                    self.reply()
                elif cmd == 0x45:                    # SET_DEVICE_EXT
                    self.read(6)
                    self.reply()
                elif cmd == 0x55:                    # LOAD_ADDRESS (word address)
                    lo, hi, _ = self.read(3)
                    self.addr = (lo | hi << 8) * 2
                    self.reply()
                elif cmd == 0x56:                    # UNIVERSAL
                    self.read(5)
                    self.reply(b"\x00")
                elif cmd == 0x64:                    # PROG_PAGE
                    hi, lo, _ = self.read(3)
                    data = self.read((hi << 8) | lo)
                    self.read(1)
                    for i, b in enumerate(data):
                        self.flash[self.addr + i] = b
                    self.reply()
                elif cmd == 0x74:                    # READ_PAGE
                    hi, lo, _ = self.read(3)
                    self.read(1)
                    n = (hi << 8) | lo
                    self.reply(bytes(self.flash.get(self.addr + i, 0xFF) for i in range(n)))
                elif cmd == 0x75:                    # READ_SIGN
                    self.read(1)
                    self.reply(self.signature)
                elif cmd in (0x30, 0x50, 0x51):      # GET_SYNC, ENTER/LEAVE_PROGMODE
                    if self.read(1)[0] == EOP:
                        self.reply()
                # anything else (line noise while avrdude resets the board): ignored
        except (EOFError, OSError):
            pass


def main():
    import argparse
    import signal
    import time
    ap = argparse.ArgumentParser(description="fake Arduino bootloader (optiboot / STK500 v1) on a pseudo-terminal")
    ap.add_argument("--port-file", required=True, help="the pty path is written here")
    ap.add_argument("--image-out", help="flash image written here on exit")
    ap.add_argument("--baud", type=int, default=0, help="only answer at this speed (57600 or 115200)")
    ap.add_argument("--signature", default=SIGNATURE.hex(), help="chip signature, hex (default ATmega328P 1e950f)")
    a = ap.parse_args()
    boot = FakeOptiboot(bytes.fromhex(a.signature), a.baud)
    tmp = a.port_file + ".tmp"
    with open(tmp, "w") as f:
        f.write(boot.port + "\n")
    os.replace(tmp, a.port_file)
    stop = []
    signal.signal(signal.SIGTERM, lambda *_: stop.append(1))
    signal.signal(signal.SIGINT, lambda *_: stop.append(1))
    while not stop:
        time.sleep(0.1)
    if a.image_out:
        tmp = a.image_out + ".tmp"
        with open(tmp, "wb") as f:
            f.write(boot.image())
        os.replace(tmp, a.image_out)
    boot.close()


if __name__ == "__main__":
    main()
