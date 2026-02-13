package entities

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type OTP struct {
	Key         string
	Created     time.Time
	IdUsuario   string
	NomeUsuario string
	HostId      string
}

type OtpMap map[string]OTP

type OtpManager struct {
	sync.Mutex
	OtpMap OtpMap
}
type ClientManager struct {
	sync.Mutex
	Clients map[string]*WsCliente //clientes conectados via ws
}
type TokenData struct {
	IdUsuario int `json:"id_usuario"`
}

type WsSocketClientes struct {
	Mu       sync.Mutex           `json:"mu"`
	Clientes map[string]WsCliente `json:"clientes"`
}

type WsCliente struct {
	WsConn    *websocket.Conn `json:"wsconn"`
	IdUsuario string          `json:"id_usuario"`
	Id        string          `json:"id"`
	sync.Mutex
}

type LoginResponse struct {
	Token         string    `json:"token"`
	DataExpiracao time.Time `json:"data_expiracao"`
}

type Usuario struct {
	NomeUsuario string `json:"nome_usuario"`
	IdUsuario   string `json:"id_usuario"`
}

type WsGrid struct {
	UltimoAlterado int            `json:"ultimo_alterado"`
	GridCores      map[int]string `json:"grid_cores"`
	sync.Mutex
}
