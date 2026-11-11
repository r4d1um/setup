"""Shared test fakes from r4d1um/setup (toolchain "fakes").

- r4fakes.optiboot.FakeOptiboot: Arduino bootloader (STK500 v1) on a pty, for the real avrdude
- r4fakes/esptool.py: stands in for esptool (FAKE_ESPTOOL_* variables, see the file)
- fake-mqtt-broker (Go binary next to these in the toolchain's bin/): MQTT 3.1.1 broker logging JSON lines
"""
