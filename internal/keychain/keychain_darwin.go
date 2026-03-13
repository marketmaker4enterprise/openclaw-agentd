//go:build darwin

package keychain

import (
	"fmt"

	gokeychain "github.com/keybase/go-keychain"
)

func darwinStore(service, account string, secret []byte) error {
	item := gokeychain.NewItem()
	item.SetSecClass(gokeychain.SecClassGenericPassword)
	item.SetService(service)
	item.SetAccount(account)
	item.SetData(secret)
	item.SetAccessible(gokeychain.AccessibleWhenUnlockedThisDeviceOnly)
	item.SetSynchronizable(gokeychain.SynchronizableNo)

	// Try adding first; if it already exists, update.
	err := gokeychain.AddItem(item)
	if err == gokeychain.ErrorDuplicateItem {
		query := gokeychain.NewItem()
		query.SetSecClass(gokeychain.SecClassGenericPassword)
		query.SetService(service)
		query.SetAccount(account)

		update := gokeychain.NewItem()
		update.SetData(secret)
		return gokeychain.UpdateItem(query, update)
	}
	return err
}

func darwinLoad(service, account string) ([]byte, error) {
	query := gokeychain.NewItem()
	query.SetSecClass(gokeychain.SecClassGenericPassword)
	query.SetService(service)
	query.SetAccount(account)
	query.SetMatchLimit(gokeychain.MatchLimitOne)
	query.SetReturnData(true)

	results, err := gokeychain.QueryItem(query)
	if err != nil {
		return nil, fmt.Errorf("keychain query: %w", err)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("secret not found in keychain: service=%s account=%s", service, account)
	}
	return results[0].Data, nil
}

func darwinDelete(service, account string) error {
	item := gokeychain.NewItem()
	item.SetSecClass(gokeychain.SecClassGenericPassword)
	item.SetService(service)
	item.SetAccount(account)
	return gokeychain.DeleteItem(item)
}
