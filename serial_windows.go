package serial

// #include <windows.h>
import "C"

import (
	"fmt"
	"syscall"
)

type port struct {
	handle syscall.Handle
}

// New allocates and returns a new serial port controller.
func New() Port {
	return &port{
		handle: syscall.InvalidHandle,
	}
}

// Open connects to the given serial port.
func (p *port) Open(c *Config) (err error) {
	handle, err := newHandle(c)
	if err != nil {
		return
	}
	// Read and write timeout
	if c.Timeout > 0 {
		timeout := C.DWORD(c.Timeout.Nanoseconds() / 1E6)
		var timeouts C.COMMTIMEOUTS
		// wait until a byte arrived or time out
		timeouts.ReadIntervalTimeout = C.MAXDWORD
		timeouts.ReadTotalTimeoutMultiplier = C.MAXDWORD
		timeouts.ReadTotalTimeoutConstant = timeout
		timeouts.WriteTotalTimeoutMultiplier = 0
		timeouts.WriteTotalTimeoutConstant = timeout
		if C.SetCommTimeouts(C.HANDLE(handle), &timeouts) == 0 {
			err = fmt.Errorf("serial: could not set timeout: %v", syscall.GetLastError())
			syscall.CloseHandle(handle)
			return
		}
	}
	p.handle = handle
	return
}

func (p *port) Close() (err error) {
	if p.handle == syscall.InvalidHandle {
		return
	}
	err = syscall.CloseHandle(p.handle)
	p.handle = syscall.InvalidHandle
	return
}

// Read reads from serial port.
// It is blocked until data received or timeout after p.timeout.
func (p *port) Read(b []byte) (n int, err error) {
	var done uint32
	if err = syscall.ReadFile(p.handle, b, &done, nil); err != nil {
		return
	}
	if done == 0 {
		err = ErrTimeout
		return
	}
	n = int(done)
	return
}

// Write writes data to the serial port.
func (p *port) Write(b []byte) (n int, err error) {
	var done uint32
	if err = syscall.WriteFile(p.handle, b, &done, nil); err != nil {
		return
	}
	n = int(done)
	return
}

func newHandle(c *Config) (handle syscall.Handle, err error) {
	handle, err = syscall.CreateFile(
		syscall.StringToUTF16Ptr(c.Address),
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		0,   // mode
		nil, // security
		syscall.OPEN_EXISTING, // create mode
		0, // attributes
		0) // templates
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			syscall.CloseHandle(handle)
		}
	}()
	var dcb C.DCB
	dcb.BaudRate = C.DWORD(c.BaudRate)
	// Data bits
	if c.DataBits == 0 {
		dcb.ByteSize = 8
	} else {
		dcb.ByteSize = C.BYTE(c.DataBits)
	}
	// Stop bits
	switch c.StopBits {
	case 0, 1:
		// Default is one stop bit.
		dcb.StopBits = C.ONESTOPBIT
	case 2:
		dcb.StopBits = C.TWOSTOPBITS
	default:
		err = fmt.Errorf("serial: unsupported stop bits %v", c.StopBits)
		return
	}
	// Parity
	switch c.Parity {
	case "", "E":
		// Default parity mode is Even.
		dcb.Parity = C.EVENPARITY
	case "O":
		dcb.Parity = C.ODDPARITY
	case "N":
		dcb.Parity = C.NOPARITY
	default:
		err = fmt.Errorf("serial: unsupported parity %v", c.Parity)
		return
	}
	if C.SetCommState(C.HANDLE(handle), &dcb) == 0 {
		err = fmt.Errorf("serial: could not set device state: %v", syscall.GetLastError())
		return
	}
	return
}
