package ws

import (
	"github.com/cploutarchou/crypto-sdk-suite/bybit/ws/client"
	"github.com/cploutarchou/crypto-sdk-suite/bybit/ws/private"
	"github.com/cploutarchou/crypto-sdk-suite/bybit/ws/public"
)

type WebSocket interface {
	Private() (private.Private, error)
	Public() (public.Public, error)
}

type implWebSocket struct {
	private private.Private
	public  public.Public
}

func (i *implWebSocket) Private() (private.Private, error) {
	return i.private, nil
}

func (i *implWebSocket) Public() (public.Public, error) {
	return i.public, nil
}
func New(publicClient, privateClient *client.Client, isTestnet bool) WebSocket {
	return &implWebSocket{
		private: private.New(privateClient, isTestnet),
		public:  public.New(publicClient, isTestnet),
	}
}
