//go:build windows

/*
Copyright (c) Jason Walton <dev@lucid.thedremaing.org> (https://www.thedreaming.org)
Copyright (c) Sindre Sorhus <sindresorhus@gmail.com> (https://sindresorhus.com)

Released under the MIT License:

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package ansi

import (
	"sync"

	"golang.org/x/sys/windows"
)

var (
	ansiErr  error
	ansiOnce sync.Once
)

func EnableANSI() error {
	ansiOnce.Do(func() {
		ansiErr = realEnableANSI()
	})
	return ansiErr
}

func realEnableANSI() error {
	// We want to enable the following modes, if they are not already set:
	// ENABLE_VIRTUAL_TERMINAL_PROCESSING on stdout (color support)
	// ENABLE_VIRTUAL_TERMINAL_INPUT on stdin (ansi input sequences)
	// See https://docs.microsoft.com/en-us/windows/console/console-virtual-terminal-sequences
	if err := windowsSetMode(windows.STD_OUTPUT_HANDLE, windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
		return err
	}
	if err := windowsSetMode(windows.STD_INPUT_HANDLE, windows.ENABLE_VIRTUAL_TERMINAL_INPUT); err != nil {
		return err
	}
	return nil
}

func windowsSetMode(stdhandle uint32, modeFlag uint32) (err error) {
	handle, err := windows.GetStdHandle(stdhandle)
	if err != nil {
		return err
	}

	// Get the existing console mode.
	var mode uint32
	err = windows.GetConsoleMode(handle, &mode)
	if err != nil {
		return err
	}

	// Enable the mode if it is not currently set
	if mode&modeFlag != modeFlag {
		mode = mode | modeFlag
		err = windows.SetConsoleMode(handle, mode)
		if err != nil {
			return err
		}
	}

	return nil
}
