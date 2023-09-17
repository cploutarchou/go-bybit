package account

import "github.com/cploutarchou/crypto-sdk-suite/bybit/client"

type Account interface {
	Wallet() *Wallet
	UpgradeToUnified() *UpgradeToUnified
	Borrow() *Borrow
	Collateral() *CollateralCoin
	CoinGreek() *CoinGreeks
	FeeRates() *FeeRates
}

type account struct {
	client *client.Client
}

func (a *account) Collateral() *CollateralCoin {
	return NewSetCollateralCoin(a.client)
}

func (a *account) Wallet() *Wallet {
	return NewWallet(a.client)
}

func (a *account) UpgradeToUnified() *UpgradeToUnified {
	return NewUpgradeToUnifiedRequest(a.client)
}

func (a *account) Borrow() *Borrow {
	return NewBorrow(a.client)
}

func (a *account) CoinGreek() *CoinGreeks {
	return NewCoinGreeks(a.client)
}

func (a *account) FeeRates() *FeeRates {
	return NewFeeRates(a.client)
}
func New(client *client.Client) Account {
	return &account{client: client}
}
