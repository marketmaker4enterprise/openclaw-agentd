//go:build !darwin

package keychain

import "fmt"

// On non-darwin, these are stubs — the real file-based fallback lives in keychain.go.

func darwinStore(service, account string, secret []byte) error {
	return fmt.Errorf("darwinStore called on non-darwin platform")
}

func darwinLoad(service, account string) ([]byte, error) {
	return nil, fmt.Errorf("darwinLoad called on non-darwin platform")
}

func darwinDelete(service, account string) error {
	return fmt.Errorf("darwinDelete called on non-darwin platform")
}
