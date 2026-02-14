package controller

import (
	"encoding/json"
	"go-ping-pong/entities"
	"go-ping-pong/model"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func WsEndpointHandler(w http.ResponseWriter, req *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	otp := req.URL.Query().Get("otp")
	if otp == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	otpClient := model.VerifyOTPWs(otp)

	WsEndpointRegistrar(w, req, otpClient)
}

func WsEndpointRegistrar(w http.ResponseWriter, r *http.Request, otpClient entities.OTP) {
	conn, err := model.WebsocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		http.Error(w, "Failed to upgrade connection", http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	uniqueId := uuid.NewString()
	newClient := &entities.WsCliente{
		Id:        uniqueId,
		WsConn:    conn,
		IdUsuario: otpClient.IdUsuario,
	}

	model.WsClientesGrid.Lock()
	model.WsClientesGrid.Clients[uniqueId] = newClient
	model.WsClientesGrid.Unlock()

	if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
		log.Println("Error sending initial message:", err)
		return
	}

	model.Reader(newClient)
}

func GetTokenWsHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if req.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if req.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var requestData entities.Usuario

	err := json.NewDecoder(req.Body).Decode(&requestData)
	if err != nil {
		http.Error(w, "Erro ao decodificar requisição: "+err.Error(), http.StatusBadRequest)
		return
	}

	if requestData.IdUsuario == "" {
		http.Error(w, "ID do usuário é obrigatório", http.StatusBadRequest)
		return
	}

	if requestData.NomeUsuario == "" {
		http.Error(w, "Nome do usuário é obrigatório", http.StatusBadRequest)
		return
	}

	generatedToken, expiresAt, err := model.GetTokenWs(requestData)
	if err != nil {
		http.Error(w, "Erro ao gerar token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := entities.LoginResponse{
		Token:         generatedToken,
		DataExpiracao: expiresAt,
	}

	json.NewEncoder(w).Encode(response)
}
