//go:build windows

package storage

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const cryptProtectUIForbidden = 0x1

func protectStrongholdBytes(plain []byte) ([]byte, error) {
	if len(plain) == 0 {
		return nil, nil
	}
	inBlob := newWindowsDataBlob(plain)
	var outBlob windows.DataBlob
	description, err := windows.UTF16PtrFromString("CialloClaw Stronghold")
	if err != nil {
		return nil, err
	}
	if err := windows.CryptProtectData(&inBlob, description, nil, 0, nil, cryptProtectUIForbidden, &outBlob); err != nil {
		return nil, fmt.Errorf("crypt protect data: %w", err)
	}
	defer freeWindowsDataBlob(outBlob)
	protected := append([]byte(nil), unsafe.Slice(outBlob.Data, outBlob.Size)...)
	return protected, nil
}

func unprotectStrongholdBytes(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, nil
	}
	inBlob := newWindowsDataBlob(ciphertext)
	var outBlob windows.DataBlob
	if err := windows.CryptUnprotectData(&inBlob, nil, nil, 0, nil, cryptProtectUIForbidden, &outBlob); err != nil {
		return nil, fmt.Errorf("crypt unprotect data: %w", err)
	}
	defer freeWindowsDataBlob(outBlob)
	decoded := append([]byte(nil), unsafe.Slice(outBlob.Data, outBlob.Size)...)
	return decoded, nil
}

func newWindowsDataBlob(data []byte) windows.DataBlob {
	if len(data) == 0 {
		return windows.DataBlob{}
	}
	return windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
}

func freeWindowsDataBlob(blob windows.DataBlob) {
	if blob.Data == nil {
		return
	}
	_, _ = windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(blob.Data))))
}
